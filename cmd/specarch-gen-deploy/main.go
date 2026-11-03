// Command specarch-gen-deploy is the plug-in behind specarch generate
// deploy: it writes, per deployment of an implementation file, the
// Compose file, the Kubernetes manifests or the cloud-init file of each
// host, as the deploy target's settings name the form, with the runbooks
// of the release, the rollback and each migration, and the script of a
// fresh environment for a Compose deployment named under fresh. It reads
// specarch's request on standard input and answers with the files on
// standard output; it never touches the disk.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/SpecArch/specarch"
	"github.com/SpecArch/specarch/internal/capability"
	"github.com/SpecArch/specarch/internal/gendeploy"
)

// capabilities is what this plug-in answers to --capabilities.
var capabilities = capability.Plugin{
	Target:  "deploy",
	Version: specarch.Version,
	Summary: "Writes each deployment of an implementation file as a Compose file, Kubernetes manifests or a cloud-init file per host, the release, rollback and migration runbooks, and a fresh environment's script that builds, loads, tests and tears down a Compose deployment.",
	Generates: []string{
		"<deployment>/compose.yaml and <deployment>/<deployment>.env for a deployment of form compose: a service per server, the database as a container of the sql target's dialect with a named volume, the links as depends_on, the settings' values in the env file, each secret as a variable Compose takes from its environment and refuses to run without",
		"<deployment>/kubernetes.yaml for a deployment of form kubernetes: a Namespace, a ConfigMap of the settings' values, a Deployment and a Service per service server, a StatefulSet with its volume claim per database server, a NetworkPolicy per server the links lead to, each secret read from the Secret <system>-secrets",
		"<deployment>/<server>.cloud-init.yaml for a deployment of form cloud-init, one per host: podman installed, the server's image run under systemd through a Quadlet unit, the settings' values in a file the unit reads, the secrets read from /etc/<system>/secrets.env, which the unit refuses to start without",
		"release.sh, rollback.sh and migrations/<name>.sh: each procedure of the deployment stage as a runbook that prints every step with its check and goes on only on yes",
		"<deployment>/fresh.sh for a compose deployment named under settings.fresh: the build task, a Compose project named for the run alone, the sql target's migrations and the seed loaded into the database, the implementation's test suite run with the settings, and the project torn down with its volumes",
	},
	DoesNotGenerate: []string{
		"a value of a secret, in any file: a secret is referenced where the form reads it from the host",
		"a container for a server that is a dependency of the design: it is reached at its URL, and a fresh environment does not stand in for it",
		"a database container for a dialect other than postgresql and mariadb",
		"the Secret object of Kubernetes, the secrets.env file of a host, or a provisioner for the hosts of a cloud-init deployment, so no fresh environment for one",
		"running any of the files against a host",
	},
}

func main() {
	if len(os.Args) > 1 {
		os.Exit(capability.Serve(os.Args[1:], os.Stdout, os.Stderr, capabilities))
	}
	os.Exit(run(os.Stdin, os.Stdout, os.Stderr))
}

func run(stdin io.Reader, stdout, stderr io.Writer) int {
	data, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-deploy: cannot read the request: %v\n", err)
		return 2
	}
	req, err := gendeploy.Decode(data)
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-deploy: the request is not specarch's JSON (%v); run this plug-in through specarch generate deploy\n", err)
		return 2
	}
	out, err := json.Marshal(gendeploy.Generate(req))
	if err != nil {
		fmt.Fprintf(stderr, "specarch-gen-deploy: cannot encode the answer: %v\n", err)
		return 2
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "specarch-gen-deploy: cannot write the answer: %v\n", err)
		return 2
	}
	return 0
}
