// Package jenkins implements some helper functions to use during
// tests. Many times certain facilities are not available, or tests
// must run differently.
package jenkins

import (
	"os"
	"strings"
)

// EnvVar is a type to use travis-only env var names with
// the type system.
type EnvVar string

// Environment variables that Jenkins uses.
const (
	VarBuildNumber    EnvVar = "BUILD_NUMBER"
	VarBuildId        EnvVar = "BUILD_ID"
	VarBuildUrl       EnvVar = "BUILD_URL"
	VarNodeName       EnvVar = "NODE_NAME"
	VarJobName        EnvVar = "JOB_NAME"
	VarBuildTag       EnvVar = "BUILD_TAG"
	VarJenkinsUrl     EnvVar = "JENKINS_URL"
	VarExecutorNumber EnvVar = "EXECUTOR_NUMBER"
	VarJavaHome       EnvVar = "JAVA_HOME"
	VarWorkspace      EnvVar = "WORKSPACE"
	VarSvnRevision    EnvVar = "SVN_REVISION"
	VarCvsBranch      EnvVar = "CVS_BRANCH"
	VarGitCommit      EnvVar = "GIT_COMMIT"
	VarGitUrl         EnvVar = "GIT_URL"
	VarGitBranch      EnvVar = "GIT_BRANCH"
)

// IsRunning attempts to determine whether this process is
// running on Jenkins CI. This is done by checking any of the
// following:
//
//  JENKINS_URL is set
//  BuildTag has prefix "jenkins-"
//
func IsRunning() bool {
	return len(Env(VarJenkinsUrl)) > 0 || strings.HasPrefix(Env(VarBuildTag), "jenkins-")
}

// Env returns the value of a travis env variable.
func Env(v EnvVar) string {
	return os.Getenv(string(v))
}

// JobName returns the jenkins JOB_NAME of this build.
func JobName() string {
	return Env(VarJobName)
}

// BuildTag returns the jenkins BUILD_TAG.
func BuildTag() string {
	return Env(VarBuildTag)
}
