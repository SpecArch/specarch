package gendeploy

import (
	"fmt"
	"strings"
)

// cloudInit writes a deployment as one cloud-init file per host: each
// server that runs something of the implementation is a host that
// installs podman and runs the server's image under systemd through a
// Quadlet unit, with the settings' values in a file the unit reads. The
// secrets are read from /etc/<system>/secrets.env, which the operator
// writes on the host and this file never holds: the unit refuses to start
// without it, naming each variable it misses.
func (g *gen) cloudInit(d *deployment) {
	etc := "/etc/" + g.sysName
	secretsFile := etc + "/secrets.env"
	for _, s := range d.servers {
		if s.dependency != "" {
			continue
		}
		image := g.imageOf(d, s)
		if image == "" {
			continue
		}
		var need []string // the variables secrets.env must hold on this host
		var unit strings.Builder
		unit.WriteString("[Unit]\n")
		unit.WriteString("Description=" + oneLine(firstNonEmpty(s.desc, s.name+" of "+g.sysName)) + "\n")
		unit.WriteString("AssertPathExists=" + secretsFile + "\n")
		unit.WriteString("[Container]\n")
		unit.WriteString("Image=" + image + "\n")
		unit.WriteString("ContainerName=" + s.name + "\n")
		dbNote := ""
		switch s.kind {
		case "service":
			unit.WriteString("EnvironmentFile=" + etc + "/" + d.key + ".env\n")
			unit.WriteString("EnvironmentFile=" + secretsFile + "\n")
			for _, sc := range g.secrets() {
				need = append(need, sc.env)
			}
		case "database":
			db, ok := g.database(d, s)
			if !ok {
				g.problem("/", "the database server %s of the deployment %s runs the sql target's dialect %s, and %s runs postgresql and mariadb as containers; write the host %s by hand, beside these files", s.name, d.key, d.dialect, name, s.name)
				dbNote = fmt.Sprintf("# the dialect %s is not run by %s: the environment and the volume of %s are written by hand\n", d.dialect, name, s.name)
				break
			}
			unit.WriteString("EnvironmentFile=" + secretsFile + "\n")
			unit.WriteString("Environment=" + db.databaseVar + "=" + dbName(g.sysName) + "\n")
			unit.WriteString("Volume=" + s.name + "-data.volume:" + db.dataPath + "\n")
			need = append(need, db.passwordVar)
		}
		for _, p := range s.ports {
			unit.WriteString(fmt.Sprintf("PublishPort=%d:%d\n", p, p))
		}
		unit.WriteString("[Service]\n")
		if len(need) > 0 {
			unit.WriteString("ExecStartPre=/bin/sh -c 'for v in " + strings.Join(need, " ") + "; do grep -q \"^$v=\" " + secretsFile + " || { echo \"$v is not in " + secretsFile + ": write it there, from the secret store of the host\" >&2; exit 1; }; done'\n")
		}
		unit.WriteString("Restart=always\n")
		unit.WriteString("[Install]\nWantedBy=multi-user.target\n")

		var b strings.Builder
		b.WriteString("#cloud-config\n")
		b.WriteString(g.header(d, fmt.Sprintf("the host %s of the deployment %s of environment %s as a cloud-init file", s.name, d.key, d.env)))
		b.WriteString("# The host runs " + s.name + " as a container under systemd, through a Quadlet unit (podman 4.4 or later). The secrets are read from " + secretsFile + ", which the operator writes on the host")
		if len(need) > 0 {
			b.WriteString(", holding " + strings.Join(need, ", "))
		}
		b.WriteString("; the unit refuses to start without it, naming what it misses, and this file never holds a value of it.\n")
		b.WriteString(dbNote)
		b.WriteString("# " + d.serverPointer(s)[2:] + "\n")
		b.WriteString("package_update: true\npackages:\n  - podman\n")
		b.WriteString("write_files:\n")
		if s.kind == "service" {
			b.WriteString("  - path: " + yamlString(etc+"/"+d.key+".env") + "\n    permissions: \"0640\"\n    content: |\n")
			b.WriteString(g.indent(g.envFile(d), "      "))
		}
		if s.kind == "database" && strings.Contains(unit.String(), "-data.volume:") {
			b.WriteString("  - path: " + yamlString("/etc/containers/systemd/"+s.name+"-data.volume") + "\n    permissions: \"0644\"\n    content: |\n")
			b.WriteString("      [Volume]\n      VolumeName=" + s.name + "-data\n")
		}
		b.WriteString("  - path: " + yamlString("/etc/containers/systemd/"+s.name+".container") + "\n    permissions: \"0644\"\n    content: |\n")
		b.WriteString(g.indent(unit.String(), "      "))
		b.WriteString("runcmd:\n")
		b.WriteString("  - " + yamlString("systemctl daemon-reload") + "\n")
		b.WriteString("  - " + yamlString("systemctl start "+s.name+".service || { echo \""+s.name+".service did not start; its reason: $(systemctl status "+s.name+".service --no-pager 2>&1 | tail -n 5)\" >&2; exit 1; }") + "\n")
		g.file(d.key+"/"+s.name+".cloud-init.yaml", b.String())
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// oneLine joins a text's lines with spaces.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
