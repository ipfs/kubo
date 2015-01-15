package jenkins

import (
	"os"
	"strings"
	"testing"
)

func TestIsRunning(t *testing.T) {
	hasPrefix := strings.HasPrefix(os.Getenv("BUILD_TAG"), "jenkins-")
	tr := len(os.Getenv("JENKINS_URL")) > 0 || hasPrefix

	if tr != IsRunning() {
		t.Error("IsRunning() does not match TRAVIS && CI env var check")
	}
}
