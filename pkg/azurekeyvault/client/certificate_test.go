/*
Copyright Sparebanken Vest

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

package client

import (
	"bytes"
	"encoding/base64"
	"testing"
)

var (
	pemTestCert = "-----BEGIN PRIVATE KEY-----\n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\nk125vXj8DvpA/GTS1kARDjKz8dET\n-----END PRIVATE KEY-----\n-----BEGIN CERTIFICATE-----\nMIIFUjCCAzqgAwIBAgIQFwNmpFLpQLWUtRrCdyrn0TANBgkqhkiG9w0BAQsFADAm\nMSQwIgYDVQQDExtjdW11bHVzLXRlc3QtY2VydC5zcHZlc3Qubm8wHhcNMTkwMjAx\nMTUzNjMxWhcNMTkwMzAxMTU0NjMxWjAmMSQwIgYDVQQDExtjdW11bHVzLXRlc3Qt\nY2VydC5zcHZlc3Qubm8wggIiMA0GCSqGSIb3DQEBAQUAA4ICDwAwggIKAoICAQCv\nOy4KydxUOW6KmMhq01IAu5Rz47U1oE6ewq0Yi5ea9CrGN7eUWLOogapoKmFFhO2s\n5SDdPt9HOkDNvh75k4B7OFhM+GaOTRubXgPEg8PV7dFFS52+3C0xORdS+wvgI2i9\neIMqbr1Y8Znw5H3pLG8DsU6Q8FCo14mvW8/ou+xKbSOzWFFaP+dNHFBCARqI+DhQ\nYJFkeg4vPd+nFGxfPH/lbbR9WN0tChOTVUJlGkJlht9/0bsVmM8xAdUS/zQ6qK8n\nKWhLpCtWyo8zKDWg5gsdcMoWYgAIXpinc1NcOyGlMv263Zhw7gB+y7JEMK2Ro3e3\nSmhSpH48CkejpIsUOBNnvr514wkLNLet9sXGZvFXs7oiTkUzgu0MFsZPVAkiYhdH\ndYdg2I9e5t4yyxbu+DSr/OvRbUtC9PrO1ncJaO7p9QcXVuRNi2wxLDeaTZgd9S6M\n2fzR2xcwq3Fxk53gDlRTXgqIM/VCPA+3vp5di+MKGK7aLyNRPxeKcsDLEPHF7MeF\nZJw21xTupEMl8w5KaBd5NiKAwxbLyV8YCZFjJG3V2MOxVAA01BAm7w3lz1/iMbKi\nPGbDA0p3cxvaLYs0RdcNfZ6+4X7al7vBXj8+Hwf/tADY648eBEjTqctVDirElCmj\nN8A0ysqldwqCr+8F8k8PUfR3yb809m8QURE7mEAPVQIDAQABo3wwejAOBgNVHQ8B\nAf8EBAMCBaAwCQYDVR0TBAIwADAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUH\nAwIwHwYDVR0jBBgwFoAUlJOHnXHhHeY+AjaPPmKFVRw3K1MwHQYDVR0OBBYEFJST\nh51x4R3mPgI2jz5ihVUcNytTMA0GCSqGSIb3DQEBCwUAA4ICAQAn/chFtfLEebP5\n5Tmb+H+eEzOXaHRonUsVriV/66htOeffkNX2b2DOIosvSwKukOkVggLFmyMKhxiq\neZkkAYyMMjjtWqbkCwoCyb8iDUQLaEovy4Pzwpm3YMVK9+o6cIf4zs3AgzaSSpbo\npq8HQbmFGrUGNEyGMclvf5VL1vCw+0jLpJ1+9b79DRY7puPG19zwWWcHk2hNV3aD\n6lWar7/pjqA9ESQhDTeUsXaFMGVm0Ez97IDI/ZVO+ia5+rIo5wAcUGKuYLIs57Wl\ndhlzMil3mz2g4STiWI+VhtPnqPot6MaWuKIN4R+kJocN365WJf2wozYgEjNFANK+\n3hO396cieWBTqyoYYZRxDxz7slD5NikixrJd50QshYCzqKiNopKsafqMHqc3JKZu\nz9tBZ25g43vdSuAwxjSab5DyYGF3Z447jdKOLUYReNnoB7nlTuW5LYfOX20F/XtC\n+4iL+IDjtAfwATruKzbLnKL9IoemLs7XMoW2qYBmCAcfHrI2F3alAar2XTA9lkDR\nMPpJf9q3VzxkPhjlvi8RPJfWLD1Kw4gMVfhao/NQv3SlhQ2rBpczP8XQOWdTNWp/\n043EPQis8+56AEHis/5+NKoNcQYZJwu2uwK0fdILcStJXR//EI04zBzWo/ULe5nc\nU0GaEMA+K/ZUHV2BxSMA3Br0IwdNvg==\n-----END CERTIFICATE-----\n"
	pfxTestCert = "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"
	derTestCert = "MIIDPjCCAiagAwIBAgIQJVHUTfH7TVCyXO649HajejANBgkqhkiG9w0BAQsFADAcMRowGAYDVQQDExFjdW11bHVzLnNwdmVzdC5ubzAeFw0xOTAyMDQxNDE0NTZaFw0xOTAzMDQxNDI0NTZaMBwxGjAYBgNVBAMTEWN1bXVsdXMuc3B2ZXN0Lm5vMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAsCYvQCbABVTAUx8G8Lg7SiNzRHCxaTPG8qp5La+SfnSPAX80u3EpVZc84vYGE9QlGWth1LmhHXfGneTAq7yMRYiBFNc0Y9O+66eZDuIErQ7RDL7N7TSyMzoXgH1xhI14x0XvkZqJwaNqJBvBzSlADNWuKBvPQ2nChKlnjuY3Zrg9KJ1A5T1KF/UW4ZKSZrwyu0fyAzfpBMWkrHa+mYv1wSL0cVDOFGvZIldCMew0gXGHY4ydM0iTW878/epTNQNgeno9M4jnFUXyoVguSH8ZjsFXBOtenIUJWoJs72zHJn5yNz2Bipu0zVrHBJXLi40FBY913/t1X5iBj7WejzUXmQIDAQABo3wwejAOBgNVHQ8BAf8EBAMCBaAwCQYDVR0TBAIwADAdBgNVHSUEFjAUBggrBgEFBQcDAQYIKwYBBQUHAwIwHwYDVR0jBBgwFoAUZptBMgsn6JCzZp4So23dBSdWfcowHQYDVR0OBBYEFGabQTILJ+iQs2aeEqNt3QUnVn3KMA0GCSqGSIb3DQEBCwUAA4IBAQAbvqmrDHz6UXbddj/VYWO/m2m5Hm2cudMfInuwnGuOzO0MtKYthkELTu+CirlQjyMya/iLKb/SZ3hRQwyJP4XBWqUm2uaTHfbrv4gpc/hMQ80n8f6hOBofLrEPogaYNGHhWMjSJXj3nDKrp2cDyusTnQkLQWaix0c2GLIif3UfGJBgptUwMgsx1kaiCzbyBW/Kv8BiQSA82ocXqqxAzHBGfKhLHZZXPdoTgEE+vwwPLM1wzvhDzDAkR96/yCwGBr53dUeXXCUh70IiJbJpGNiVc33QEVYw0+Gua7rj99LK4EljlY1E1xbPadSFYeK9KsDlmb9ota2p7iKg1D1JiydK"
)

func TestImportPfx(t *testing.T) {
	pfxRaw, _ := base64.StdEncoding.DecodeString(pfxTestCert)
	cert, err := NewCertificateFromPfx(pfxRaw)
	if err != nil {
		t.Error(err)
	}
	if !cert.HasPrivateKey {
		t.Error("Certificate has no private key")
	}
	if cert.PrivateKeyType != CertificateKeyTypeRsa {
		t.Errorf("Certificate type is incorrect. Exprected '%s', but got '%s'", CertificateKeyTypeRsa, cert.PrivateKeyType)
	}
	if cert.PrivateKeyRsa == nil {
		t.Error("Private key for RSA is nil")
	}
	if len(cert.Certificates) != 1 {
		t.Errorf("Expected 1 public certificate, but found %d", len(cert.Certificates))
	}
}

func TestImportPem(t *testing.T) {
	cert, err := NewCertificateFromPem(pemTestCert)
	if err != nil {
		t.Error(err)
	}
	if !cert.HasPrivateKey {
		t.Error("Certificate has no private key")
	}
	if cert.PrivateKeyType != CertificateKeyTypeRsa {
		t.Errorf("Certificate type is incorrect. Exprected '%s', but got '%s'", CertificateKeyTypeRsa, cert.PrivateKeyType)
	}
	if cert.PrivateKeyRsa == nil {
		t.Error("Private key for RSA is nil")
	}
	if len(cert.Certificates) != 1 {
		t.Errorf("Expected 1 public certificate, but found %d", len(cert.Certificates))
	}
}

func TestImportDer(t *testing.T) {
	certRaw, _ := base64.StdEncoding.DecodeString(derTestCert)
	cert, err := NewCertificateFromDer(certRaw)
	if err != nil {
		t.Error(err)
	}
	if cert.HasPrivateKey {
		t.Error("Certificate should be public with no private key")
	}
	if len(cert.Certificates) != 1 {
		t.Errorf("Expected 1 public certificate, but found %d", len(cert.Certificates))
	}
}

func TestGetPrivateKeyPem(t *testing.T) {
	pfxRaw, _ := base64.StdEncoding.DecodeString(pfxTestCert)
	cert, err := NewCertificateFromPfx(pfxRaw)
	if err != nil {
		t.Error(err)
	}
	pemCert, err := cert.ExportPrivateKeyAsPem()
	if err != nil {
		t.Error(err)
	}

	if len(pemCert) == 0 {
		t.Error("Pem is empty")
	}
}

func TestGetPublicKeyPem(t *testing.T) {
	pfxRaw, _ := base64.StdEncoding.DecodeString(pfxTestCert)
	cert, err := NewCertificateFromPfx(pfxRaw)
	if err != nil {
		t.Error(err)
	}
	pemCert, err := cert.ExportPublicKeyAsPem()
	if err != nil {
		t.Error(err)
	}

	if len(pemCert) == 0 {
		t.Error("Pem is empty")
	}
}

func TestGetRawCert(t *testing.T) {
	pfxRaw, _ := base64.StdEncoding.DecodeString(pfxTestCert)
	cert, err := NewCertificateFromPfx(pfxRaw)
	if err != nil {
		t.Error(err)
	}
	rawCert := cert.ExportRaw()
	if err != nil {
		t.Error(err)
	}

	if !bytes.Equal(pfxRaw, rawCert) {
		t.Error("Original cert does not match exported raw cert")
	}
}
