package gendeploy

import (
	"fmt"
	"strings"
)

// compose writes a deployment as a Compose file beside the file of its
// settings' values: one service per server that runs something of the
// implementation, the database as a container of the sql target's
// dialect with a named volume, the links as depends_on, and each secret
// as a variable Compose takes from the environment it runs in and refuses
// to run without.
func (g *gen) compose(d *deployment) {
	var b strings.Builder
	b.WriteString(g.header(d, fmt.Sprintf("the deployment %s of environment %s as a Compose file", d.key, d.env)))
	b.WriteString("# A secret is a variable of the environment Compose runs in, ${NAME:?reason}: it is read from the host's secret store and never written here.\n")
	b.WriteString("name: " + yamlString(g.sysName+"-"+d.key) + "\n")
	b.WriteString("# specarch: #/environments/" + d.env + "\n")
	b.WriteString("services:\n")
	var volumes []string
	for _, s := range d.servers {
		if s.dependency != "" {
			b.WriteString("  " + dependencyNote(s))
			continue
		}
		image := g.imageOf(d, s)
		if image == "" {
			b.WriteString(fmt.Sprintf("  # %s: no image under settings.images.%s of the deploy target; not written\n", s.name, s.name))
			continue
		}
		b.WriteString("  " + d.serverPointer(s) + "\n")
		b.WriteString("  " + s.name + ":\n")
		b.WriteString("    image: " + yamlString(image) + "\n")
		switch s.kind {
		case "service":
			b.WriteString("    env_file:\n      - " + d.key + ".env\n")
			if secrets := g.secrets(); len(secrets) > 0 {
				b.WriteString("    environment:\n")
				for _, sc := range secrets {
					b.WriteString("      # specarch: #" + sc.pointer + "\n")
					b.WriteString("      " + sc.env + ": " + yamlString("${"+sc.env+":?"+secretReason(sc)+"; set it in the environment Compose runs in}") + "\n")
				}
			}
		case "database":
			db, ok := g.database(d, s)
			if !ok {
				g.problem("/", "the database server %s of the deployment %s runs the sql target's dialect %s, and %s runs postgresql and mariadb as containers; write the service %s by hand, beside this file", s.name, d.key, d.dialect, name, s.name)
				b.WriteString(fmt.Sprintf("    # the dialect %s is not run by %s: the environment, the volume and the readiness of %s are written by hand\n", d.dialect, name, s.name))
				break
			}
			pw := upperSnake(s.name) + "_PASSWORD"
			b.WriteString("    environment:\n")
			b.WriteString("      " + db.passwordVar + ": " + yamlString("${"+pw+":?the password of the database "+s.name+", from the host's secret store; set it in the environment Compose runs in}") + "\n")
			b.WriteString("      " + db.databaseVar + ": " + yamlString(dbName(g.sysName)) + "\n")
			b.WriteString("    volumes:\n      - " + s.name + "-data:" + db.dataPath + "\n")
			volumes = append(volumes, s.name+"-data")
		}
		if len(s.ports) > 0 {
			b.WriteString("    ports:\n")
			for _, p := range s.ports {
				b.WriteString(fmt.Sprintf("      - %q\n", fmt.Sprintf("%d:%d", p, p)))
			}
		}
		if deps := d.dependsOn(s.name); len(deps) > 0 {
			b.WriteString("    depends_on:\n")
			for _, dep := range deps {
				b.WriteString("      - " + dep + "\n")
			}
		}
	}
	if len(volumes) > 0 {
		b.WriteString("volumes:\n")
		for _, v := range volumes {
			b.WriteString("  " + v + ": {}\n")
		}
	}
	g.file(d.key+"/compose.yaml", b.String())
	g.file(d.key+"/"+d.key+".env", g.envFile(d))
}

// fresh writes the script that builds a fresh environment of a Compose
// deployment, loads it, runs the test suite there and tears it down.
func (g *gen) fresh(d *deployment) {
	if d.build == "" {
		g.problem("/", "settings.fresh of the deploy target names %s, and a fresh environment starts with the build, which the file gives neither under deployments.%s.build.task nor as the task build; name the task that builds the image", d.key, d.key)
		return
	}
	if d.test == "" {
		g.problem("/", "settings.fresh of the deploy target names %s, and a fresh environment runs the implementation's test suite, which the file gives under testing.run; write it", d.key)
		return
	}
	var dbs server
	var db database
	for _, s := range d.servers {
		if s.kind == "database" && s.dependency == "" {
			if x, ok := g.database(d, s); ok {
				dbs, db = s, x
			}
		}
	}
	secrets := g.secrets()
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	b.WriteString(g.header(d, fmt.Sprintf("a fresh environment of the deployment %s of environment %s", d.key, d.env)))
	b.WriteString("# A fresh environment: built from nothing under a project name of its own, loaded with the migrations and the seed, tested with the implementation's suite, and torn down with its volumes, so no run touches what an earlier one left.\n")
	b.WriteString("# Usage: fresh.sh <project>, where <project> is the folder the implementation file's tasks run in.\n")
	b.WriteString("# Read from the environment: COMPOSE, the compose command (podman compose when unset)")
	if dbs.name != "" {
		b.WriteString("; " + upperSnake(dbs.name) + "_PASSWORD, the throwaway database's password for this run")
	}
	for _, sc := range secrets {
		b.WriteString("; " + sc.env + ", the value of " + sc.key + " for this run")
	}
	if dbs.name != "" {
		b.WriteString(". The database is reached on the project's network as " + dbs.name + ".")
	}
	b.WriteString("\nset -eu\n")
	b.WriteString("project=${1:?usage: fresh.sh <project>, the folder the implementation file's tasks run in}\n")
	b.WriteString("here=$(cd \"$(dirname \"$0\")\" && pwd)\n")
	b.WriteString("compose_cmd=${COMPOSE:-podman compose}\n")
	if dbs.name != "" {
		pw := upperSnake(dbs.name) + "_PASSWORD"
		b.WriteString(": \"${" + pw + ":?the throwaway database's password for this run; set it, and name it in the secret that reaches the database}\"\n")
	}
	for _, sc := range secrets {
		b.WriteString("# specarch: #" + sc.pointer + "\n")
		b.WriteString(": \"${" + sc.env + ":?" + secretReason(sc) + "; set its value for this run}\"\n")
	}
	b.WriteString("run=$(date -u +%Y%m%dT%H%M%SZ)-$$\n")
	b.WriteString("name=" + g.sysName + "-fresh-$run\n")
	b.WriteString("compose() { $compose_cmd -p \"$name\" -f \"$here/compose.yaml\" \"$@\"; }\n")
	total := 6
	b.WriteString(fmt.Sprintf("printf 'fresh: 1 of %d: build in %%s: %%s\\n' \"$project\" %s\n", total, shQuote(d.build)))
	b.WriteString("(cd \"$project\" && " + d.build + ")\n")
	b.WriteString(fmt.Sprintf("echo \"fresh: 2 of %d: start the project $name\"\n", total))
	b.WriteString(fmt.Sprintf("trap 'echo \"fresh: %d of %d: tear down $name with its volumes\"; compose down --volumes --remove-orphans' EXIT\n", total, total))
	b.WriteString("compose up -d\n")
	if dbs.name != "" {
		b.WriteString(fmt.Sprintf("echo \"fresh: 3 of %d: wait for the database %s\"\n", total, dbs.name))
		b.WriteString("i=0\n")
		b.WriteString("until compose exec -T " + dbs.name + " sh -c " + shQuote(db.ready) + " >/dev/null 2>&1; do\n")
		b.WriteString("  i=$((i + 1))\n")
		b.WriteString("  [ \"$i\" -lt 60 ] || { echo \"fresh: the database " + dbs.name + " did not answer within 60 s; its log: $(compose logs " + dbs.name + " 2>&1 | tail -n 20)\" >&2; exit 1; }\n")
		b.WriteString("  sleep 1\n")
		b.WriteString("done\n")
		b.WriteString(fmt.Sprintf("echo \"fresh: 4 of %d: load the migrations and the seed into %s\"\n", total, dbs.name))
		load := "compose exec -T " + dbs.name + " sh -c " + shQuote(db.load)
		if d.migrations != "" {
			b.WriteString("for f in \"$here\"/" + d.migrations + "/*.sql; do\n")
			b.WriteString("  [ -e \"$f\" ] || { echo \"fresh: no migration under $here/" + d.migrations + "; run specarch generate sql first\" >&2; exit 1; }\n")
			b.WriteString("  echo \"fresh:   $f\"\n")
			b.WriteString("  " + load + " <\"$f\"\n")
			b.WriteString("done\n")
		} else {
			b.WriteString("# the file has no sql target, so no migration is loaded\n")
		}
		if d.seed != "" {
			b.WriteString("echo \"fresh:   $here/" + d.seed + "/seed.sql\"\n")
			b.WriteString(load + " <\"$here/" + d.seed + "/seed.sql\"\n")
		} else {
			b.WriteString("# the file has no seed target, so no seed is loaded\n")
		}
	} else {
		b.WriteString(fmt.Sprintf("echo \"fresh: 3 of %d: no database server in the deployment %s; nothing to wait for\"\n", total, d.key))
		b.WriteString(fmt.Sprintf("echo \"fresh: 4 of %d: no database server in the deployment %s; nothing to load\"\n", total, d.key))
	}
	b.WriteString(fmt.Sprintf("printf 'fresh: 5 of %d: run the test suite in %%s with the settings of %s: %%s\\n' \"$project\" %s\n", total, d.key, shQuote(d.test)))
	b.WriteString("set -a\n. \"$here/" + d.key + ".env\"\nset +a\n")
	b.WriteString("(cd \"$project\" && " + d.test + ")\n")
	b.WriteString("echo \"fresh: the suite passed on $name\"\n")
	g.file(d.key+"/fresh.sh", b.String())
}
