package gendeploy

import (
	"fmt"
	"strings"
)

// kubernetes writes a deployment as one file of Kubernetes manifests: a
// Namespace, a ConfigMap of the settings' values, a Deployment and a
// Service per service server, a StatefulSet with its volume claim and a
// Service per database server, and a NetworkPolicy per server the links
// lead to, allowing those links alone. A secret is read from the Secret
// <system>-secrets, which the operator makes and this file never writes.
func (g *gen) kubernetes(d *deployment) {
	ns := g.sysName + "-" + d.key
	settings := g.sysName + "-settings"
	secretsName := g.sysName + "-secrets"
	var b strings.Builder
	b.WriteString(g.header(d, fmt.Sprintf("the deployment %s of environment %s as Kubernetes manifests", d.key, d.env)))
	b.WriteString("# A secret is read from the Secret " + secretsName + " in the namespace " + ns + ", by the key the setting's variable names; the operator makes that Secret from the host's secret store, and this file never writes it.\n")
	b.WriteString("# specarch: #/environments/" + d.env + "\n")
	b.WriteString("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + yamlString(ns) + "\n")
	b.WriteString("---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + yamlString(settings) + "\n  namespace: " + yamlString(ns) + "\n")
	if len(d.values) > 0 {
		b.WriteString("data:\n")
		for _, v := range d.values {
			b.WriteString("  # specarch: #" + v.s.pointer + "\n")
			b.WriteString(g.indent(g.marks(v.s.pointer), "  "))
			b.WriteString("  " + v.s.env + ": " + yamlString(v.val) + "\n")
		}
	} else {
		b.WriteString("data: {}\n")
	}
	for _, s := range d.missing {
		b.WriteString("# specarch: #" + s.pointer + "\n")
		b.WriteString(g.marks(s.pointer))
		b.WriteString("# " + s.env + ": no value in the deployment " + d.key + " and no default\n")
	}
	for _, s := range g.secrets() {
		b.WriteString("# specarch: #" + s.pointer + "\n")
		b.WriteString(g.marks(s.pointer))
		b.WriteString("# " + s.env + ": " + secretReason(s) + "; a key of the Secret " + secretsName + "\n")
	}
	for _, s := range d.servers {
		if s.dependency != "" {
			b.WriteString(dependencyNote(s))
			continue
		}
		image := g.imageOf(d, s)
		if image == "" {
			b.WriteString(fmt.Sprintf("# %s: no image under settings.images.%s of the deploy target; not written\n", s.name, s.name))
			continue
		}
		b.WriteString("---\n" + d.serverPointer(s) + "\n")
		switch s.kind {
		case "database":
			db, ok := g.database(d, s)
			if !ok {
				g.problem("/", "the database server %s of the deployment %s runs the sql target's dialect %s, and %s runs postgresql and mariadb as containers; write the StatefulSet %s by hand, beside this file", s.name, d.key, d.dialect, name, s.name)
				b.WriteString(fmt.Sprintf("# the dialect %s is not run by %s: the StatefulSet %s is written by hand\n", d.dialect, name, s.name))
				continue
			}
			if s.storage == "" {
				g.problem("/", "the database server %s of the deployment %s has no storage under settings.storage.%s of the deploy target; name the size of its volume claim, such as 20Gi, and the StatefulSet is written", s.name, d.key, s.name)
				b.WriteString(fmt.Sprintf("# %s: no storage under settings.storage.%s of the deploy target; the StatefulSet is not written\n", s.name, s.name))
				continue
			}
			b.WriteString("apiVersion: apps/v1\nkind: StatefulSet\n")
			b.WriteString(g.k8sMeta(s.name, ns))
			b.WriteString("spec:\n  serviceName: " + yamlString(s.name) + "\n  replicas: 1\n  selector:\n    matchLabels:\n      app: " + yamlString(s.name) + "\n")
			b.WriteString("  template:\n    metadata:\n      labels:\n        app: " + yamlString(s.name) + "\n    spec:\n      containers:\n")
			b.WriteString("        - name: " + yamlString(s.name) + "\n          image: " + yamlString(image) + "\n")
			b.WriteString("          env:\n")
			b.WriteString("            - name: " + db.passwordVar + "\n              valueFrom:\n                secretKeyRef:\n                  name: " + yamlString(secretsName) + "\n                  key: " + upperSnake(s.name) + "_PASSWORD\n")
			b.WriteString("            - name: " + db.databaseVar + "\n              value: " + yamlString(dbName(g.sysName)) + "\n")
			g.k8sPorts(&b, s, "          ")
			b.WriteString("          volumeMounts:\n            - name: data\n              mountPath: " + yamlString(db.dataPath) + "\n")
			b.WriteString("  volumeClaimTemplates:\n    - metadata:\n        name: data\n      spec:\n        accessModes:\n          - ReadWriteOnce\n        resources:\n          requests:\n            storage: " + yamlString(s.storage) + "\n")
		default:
			b.WriteString("apiVersion: apps/v1\nkind: Deployment\n")
			b.WriteString(g.k8sMeta(s.name, ns))
			b.WriteString("spec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: " + yamlString(s.name) + "\n")
			b.WriteString("  template:\n    metadata:\n      labels:\n        app: " + yamlString(s.name) + "\n    spec:\n      containers:\n")
			b.WriteString("        - name: " + yamlString(s.name) + "\n          image: " + yamlString(image) + "\n")
			if s.kind == "service" {
				b.WriteString("          envFrom:\n            - configMapRef:\n                name: " + yamlString(settings) + "\n")
				if secrets := g.secrets(); len(secrets) > 0 {
					b.WriteString("          env:\n")
					for _, sc := range secrets {
						b.WriteString("            # specarch: #" + sc.pointer + "\n")
						b.WriteString("            - name: " + sc.env + "\n              valueFrom:\n                secretKeyRef:\n                  name: " + yamlString(secretsName) + "\n                  key: " + sc.env + "\n")
					}
				}
			}
			g.k8sPorts(&b, s, "          ")
		}
		if len(s.ports) > 0 {
			b.WriteString("---\napiVersion: v1\nkind: Service\n")
			b.WriteString(g.k8sMeta(s.name, ns))
			b.WriteString("spec:\n  selector:\n    app: " + yamlString(s.name) + "\n  ports:\n")
			for _, p := range s.ports {
				b.WriteString(fmt.Sprintf("    - name: %s\n      port: %d\n      targetPort: %d\n", yamlString(portName(s, p)), p, p))
			}
		}
		if in := d.linksTo(s.name); len(in) > 0 {
			b.WriteString("---\n# the links into " + s.name + ": what may reach it, and on which port; nothing else does\n")
			b.WriteString("apiVersion: networking.k8s.io/v1\nkind: NetworkPolicy\n")
			b.WriteString(g.k8sMeta(s.name, ns))
			b.WriteString("spec:\n  podSelector:\n    matchLabels:\n      app: " + yamlString(s.name) + "\n  policyTypes:\n    - Ingress\n  ingress:\n")
			for _, l := range in {
				b.WriteString("    - from:\n        - podSelector:\n            matchLabels:\n              app: " + yamlString(l.from) + "\n")
				b.WriteString(fmt.Sprintf("      ports:\n        - port: %d\n", l.port))
			}
		}
	}
	g.file(d.key+"/kubernetes.yaml", b.String())
}

func (g *gen) k8sMeta(name, ns string) string {
	return "metadata:\n  name: " + yamlString(name) + "\n  namespace: " + yamlString(ns) + "\n  labels:\n    app: " + yamlString(name) + "\n"
}

func (g *gen) k8sPorts(b *strings.Builder, s server, indent string) {
	if len(s.ports) == 0 {
		return
	}
	b.WriteString(indent + "ports:\n")
	for _, p := range s.ports {
		b.WriteString(fmt.Sprintf("%s  - name: %s\n%s    containerPort: %d\n", indent, yamlString(portName(s, p)), indent, p))
	}
}

// portName names a port by its protocol, or by its number when the
// server names none.
func portName(s server, p int) string {
	if pr := s.protocols[p]; pr != "" {
		return pr
	}
	return fmt.Sprintf("port-%d", p)
}

// indent puts a prefix before each line of a text.
func (g *gen) indent(text, prefix string) string {
	if text == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n") + "\n"
}
