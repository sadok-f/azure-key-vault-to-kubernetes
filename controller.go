/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	azureKeyVaultSecretv1alpha1 "github.com/SparebankenVest/azure-keyvault-controller/pkg/apis/azurekeyvaultcontroller/v1alpha1"
	clientset "github.com/SparebankenVest/azure-keyvault-controller/pkg/client/clientset/versioned"
	keyvaultScheme "github.com/SparebankenVest/azure-keyvault-controller/pkg/client/clientset/versioned/scheme"
	informers "github.com/SparebankenVest/azure-keyvault-controller/pkg/client/informers/externalversions/azurekeyvaultcontroller/v1alpha1"
	listers "github.com/SparebankenVest/azure-keyvault-controller/pkg/client/listers/azurekeyvaultcontroller/v1alpha1"
)

const controllerAgentName = "azure-keyvault-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a AzureKeyVaultSecret is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a AzureKeyVaultSecret fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by AzureKeyVaultSecret"
	// MessageResourceSynced is the message used for an Event fired when a AzureKeyVaultSecret
	// is synced successfully
	MessageResourceSynced = "AzureKeyVaultSecret synced successfully"
)

// Controller is the controller implementation for AzureKeyVaultSecret resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// azureKeyvaultClientset is a clientset for our own API group
	azureKeyvaultClientset clientset.Interface

	secretsLister              corelisters.SecretLister
	secretsSynced              cache.InformerSynced
	azureKeyVaultSecretsLister listers.AzureKeyVaultSecretLister
	azureKeyVaultSecretsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue      workqueue.RateLimitingInterface
	workqueueAzure workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new AzureKeyVaultSecret controller
func NewController(kubeclientset kubernetes.Interface, azureKeyvaultClientset clientset.Interface, secretInformer coreinformers.SecretInformer, azureKeyVaultSecretsInformer informers.AzureKeyVaultSecretInformer) *Controller {
	// Create event broadcaster
	// Add azure-keyvault-controller types to the default Kubernetes Scheme so Events can be
	// logged for azure-keyvault-controller types.
	utilruntime.Must(keyvaultScheme.AddToScheme(scheme.Scheme))
	klog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeclientset:              kubeclientset,
		azureKeyvaultClientset:     azureKeyvaultClientset,
		secretsLister:              secretInformer.Lister(),
		secretsSynced:              secretInformer.Informer().HasSynced,
		azureKeyVaultSecretsLister: azureKeyVaultSecretsInformer.Lister(),
		azureKeyVaultSecretsSynced: azureKeyVaultSecretsInformer.Informer().HasSynced,
		workqueue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AzureKeyVaultSecrets"),
		workqueueAzure:             workqueue.NewNamedRateLimitingQueue(workqueue.NewItemFastSlowRateLimiter(time.Minute, time.Minute*5, 5), "AzureKeyVault"),
		recorder:                   recorder,
	}

	log.Printf("Setting up event handlers")
	// Set up an event handler for when AzureKeyVaultSecret resources change
	azureKeyVaultSecretsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueAzureKeyVaultSecret,
		UpdateFunc: func(old, new interface{}) {
			newSecret := new.(*azureKeyVaultSecretv1alpha1.AzureKeyVaultSecret)
			oldSecret := old.(*azureKeyVaultSecretv1alpha1.AzureKeyVaultSecret)
			if newSecret.ResourceVersion == oldSecret.ResourceVersion {
				// Check if secret has changed in Azure
				controller.enqueueAzurePoll(new)
				return
			}
			controller.enqueueAzureKeyVaultSecret(new)
		},
		DeleteFunc: controller.enqueueDeleteAzureKeyVaultSecret,
	})

	// Set up an event handler for when Secret resources change. This
	// handler will lookup the owner of the given Secret, and if it is
	// owned by a AzureKeyVaultSecret resource will enqueue that Secret resource for
	// processing. This way, we don't need to implement custom logic for
	// handling AzureKeyVaultSecret resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Printf("New Secret added. Handling.")
			controller.handleObject(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			newSecret := new.(*corev1.Secret)
			oldSecret := old.(*corev1.Secret)
			if newSecret.ResourceVersion == oldSecret.ResourceVersion {
				// Periodic resync will send update events for all known Secrets.
				// Two different versions of the same Secret will always have different RVs.
				return
			}
			log.Printf("warning: Secret controlled by AzureKeyVaultSecret changed. Adding to queue.")
			controller.handleObject(new)
		},
		DeleteFunc: func(obj interface{}) {
			log.Printf("Secret deleted. Handling.")
			controller.handleObject(obj)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	defer c.workqueueAzure.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Printf("Starting AzureKeyVaultSecret controller")

	// Wait for the caches to be synced before starting workers
	log.Printf("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.secretsSynced, c.azureKeyVaultSecretsSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	log.Printf("Starting workers")
	// Launch two workers to process AzureKeyVaultSecret resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
		go wait.Until(c.runAzureWorker, time.Second, stopCh)
	}

	log.Printf("Started workers")
	<-stopCh
	log.Printf("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem(c.workqueue, false) {
	}
}

func (c *Controller) runAzureWorker() {
	for c.processNextWorkItem(c.workqueueAzure, true) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(queue workqueue.RateLimitingInterface, syncAzure bool) bool {
	obj, shutdown := queue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer queue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			queue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// AzureKeyVaultSecret resource to be synced.
		if err := c.syncHandler(key, syncAzure); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			queue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		queue.Forget(obj)
		log.Printf("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the AzureKeyVaultSecret resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string, pollAzure bool) error {
	// Convert the namespace/name string into a distinct namespace and name
	log.Printf("Checking state for %s", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the AzureKeyVaultSecret resource with this namespace/name
	azureKeyVaultSecret, err := c.azureKeyVaultSecretsLister.AzureKeyVaultSecrets(namespace).Get(name)
	if err != nil {
		// The AzureKeyVaultSecret resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("AzureKeyVaultSecret '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	secretName := azureKeyVaultSecret.Spec.OutputSecret.Name
	if secretName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		utilruntime.HandleError(fmt.Errorf("%s: secret name must be specified", key))
		return nil
	}

	// Get the secret with the name specified in AzureKeyVaultSecret.spec
	secret, getSecretErr := c.secretsLister.Secrets(azureKeyVaultSecret.Namespace).Get(secretName)

	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(getSecretErr) {
		// Get secret form Azure
		secret, getSecretErr = c.kubeclientset.CoreV1().Secrets(azureKeyVaultSecret.Namespace).Create(newSecret(azureKeyVaultSecret, nil))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if getSecretErr != nil {
		return getSecretErr
	}

	// If the Secret is not controlled by this AzureKeyVaultSecret resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(secret, azureKeyVaultSecret) { // checks if the object has a controllerRef set to the given owner
		msg := fmt.Sprintf(MessageResourceExists, secret.Name)
		c.recorder.Event(azureKeyVaultSecret, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	if pollAzure {
		// Get secret form Azure
		secretValue, err := GetSecret(azureKeyVaultSecret)
		if err != nil {
			log.Printf("failed to get secret from Azure Key Vault, Error: %+v", err)
			return err
		}

		// If hash on the AzureKeyVaultSecret resource is specified, and
		// it is not equal the current hash on the Secret, we
		// should update the AzureKeyVaultSecret resource.
		secretHash := getMD5Hash(secretValue)

		if azureKeyVaultSecret.Status.SecretHash != secretHash {
			log.Printf("secret has changed in Azure Key Vault for AzureKeyvVaultSecret %s. Updating Secret now.", name)
			secret, err = c.kubeclientset.CoreV1().Secrets(azureKeyVaultSecret.Namespace).Update(newSecret(azureKeyVaultSecret, &secretValue))

			// If an error occurs during Update, we'll requeue the item so we can
			// attempt processing again later. THis could have been caused by a
			// temporary network failure, or any other transient reason.
			if err != nil {
				log.Printf("failed to create Secret, Error: %+v", err)
				return err
			}
		}
	}

	// Finally, we update the status block of the AzureKeyVaultSecret resource to reflect the
	// current state of the world
	err = c.updateAzureKeyVaultSecretStatus(azureKeyVaultSecret, secret)
	if err != nil {
		return err
	}

	c.recorder.Event(azureKeyVaultSecret, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateAzureKeyVaultSecretStatus(azureKeyVaultSecret *azureKeyVaultSecretv1alpha1.AzureKeyVaultSecret, secret *corev1.Secret) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	azureKeyVaultSecretCopy := azureKeyVaultSecret.DeepCopy()
	secretValue := string(secret.Data[azureKeyVaultSecret.Spec.OutputSecret.KeyName])
	secretHash := getMD5Hash(secretValue)
	azureKeyVaultSecretCopy.Status.SecretHash = secretHash

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the AzureKeyVaultSecret resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.azureKeyvaultClientset.AzurekeyvaultcontrollerV1alpha1().AzureKeyVaultSecrets(azureKeyVaultSecret.Namespace).Update(azureKeyVaultSecretCopy)
	return err
}

// enqueueAzureKeyVaultSecret takes a AzureKeyVaultSecret resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than AzureKeyVaultSecret.
func (c *Controller) enqueueAzureKeyVaultSecret(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// enqueueAzureKeyVaultSecret takes a AzureKeyVaultSecret resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than AzureKeyVaultSecret.
func (c *Controller) enqueueAzurePoll(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueueAzure.AddRateLimited(key)
}

// dequeueAzureKeyVaultSecret takes a AzureKeyVaultSecret resource and converts it into a namespace/name
// string which is then put onto the work queue for deltion. This method should *not* be
// passed resources of any type other than AzureKeyVaultSecret.
func (c *Controller) enqueueDeleteAzureKeyVaultSecret(obj interface{}) {
	var key string
	var err error
	if key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
	c.workqueueAzure.Forget(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the AzureKeyVaultSecret resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that AzureKeyVaultSecret resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		log.Printf("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	log.Printf("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a AzureKeyVaultSecret, we should not do anything more
		// with it.
		if ownerRef.Kind != "AzureKeyVaultSecret" {
			return
		}

		azureKeyVaultSecret, err := c.azureKeyVaultSecretsLister.AzureKeyVaultSecrets(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			log.Printf("ignoring orphaned object '%s' of azureKeyVaultSecret '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueAzureKeyVaultSecret(azureKeyVaultSecret)
		return
	}
}

// newSecret creates a new Secret for a AzureKeyVaultSecret resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the AzureKeyVaultSecret resource that 'owns' it.
func newSecret(azureKeyVaultSecret *azureKeyVaultSecretv1alpha1.AzureKeyVaultSecret, azureSecretValue *string) *corev1.Secret {
	var secretValue string

	if azureSecretValue == nil {
		var err error
		secretValue, err = GetSecret(azureKeyVaultSecret)
		if err != nil {
			log.Printf("failed to get secret from Azure Key Vault, Error: %+v", err)
			return nil
		}
	} else {
		secretValue = *azureSecretValue
	}

	stringData := make(map[string]string)
	stringData[azureKeyVaultSecret.Spec.OutputSecret.KeyName] = secretValue

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      azureKeyVaultSecret.Spec.OutputSecret.Name,
			Namespace: azureKeyVaultSecret.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(azureKeyVaultSecret, schema.GroupVersionKind{
					Group:   azureKeyVaultSecretv1alpha1.SchemeGroupVersion.Group,
					Version: azureKeyVaultSecretv1alpha1.SchemeGroupVersion.Version,
					Kind:    "AzureKeyVaultSecret",
				}),
			},
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: stringData,
	}
}

func getMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}