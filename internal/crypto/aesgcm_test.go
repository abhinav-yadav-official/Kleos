package crypto

import "testing"

func TestAESGCMEncryptDecryptRoundTrip(t *testing.T) {
	codec, err := NewAESGCM("00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatalf("NewAESGCM returned error: %v", err)
	}

	ciphertext, nonce, err := codec.EncryptString("smtp-password")
	if err != nil {
		t.Fatalf("EncryptString returned error: %v", err)
	}
	if string(ciphertext) == "smtp-password" {
		t.Fatal("ciphertext should not equal plaintext")
	}

	plaintext, err := codec.DecryptString(ciphertext, nonce)
	if err != nil {
		t.Fatalf("DecryptString returned error: %v", err)
	}
	if plaintext != "smtp-password" {
		t.Fatalf("plaintext = %q, want smtp-password", plaintext)
	}
}
