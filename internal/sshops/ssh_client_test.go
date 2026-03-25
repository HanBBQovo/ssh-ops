package sshops

import "testing"

func TestBuildAuthMethodsAddsKeyboardInteractiveForPassword(t *testing.T) {
	host := &HostConfig{
		ID:      "prod",
		Address: "203.0.113.10",
		User:    "root",
		Auth: AuthConfig{
			Password: "secret",
		},
		HostKey: HostKeyConfig{
			Mode: "insecure_ignore",
		},
	}

	methods, err := buildAuthMethods(host)
	if err != nil {
		t.Fatalf("buildAuthMethods() error = %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected password auth to register 2 methods, got %d", len(methods))
	}
}

func TestPasswordKeyboardInteractiveRepeatsPasswordForPrompts(t *testing.T) {
	challenge := passwordKeyboardInteractive("secret")

	answers, err := challenge("root", "Password authentication", []string{
		"Password: ",
		"Verification code: ",
	}, []bool{false, false})
	if err != nil {
		t.Fatalf("challenge() error = %v", err)
	}
	if len(answers) != 2 {
		t.Fatalf("expected 2 answers, got %d", len(answers))
	}
	for i, answer := range answers {
		if answer != "secret" {
			t.Fatalf("answer %d = %q, want secret", i, answer)
		}
	}
}
