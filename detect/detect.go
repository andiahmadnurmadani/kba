package detect

import (
	"os/exec"
	"runtime"
	"strings"
)

type Service string

const (
	ServiceMySQL    Service = "mysql"
	ServicePostgres Service = "postgres"
	ServiceMongoDB  Service = "mongodb"
	ServicePM2      Service = "pm2"
	ServiceDocker   Service = "docker"
	ServiceNginx    Service = "nginx"
	ServiceApache   Service = "apache"
	ServiceCaddy    Service = "caddy"
)

type Detection struct {
	Service  Service
	Present  bool
	Version  string
	Commands []string
}

type Result struct {
	OS        string
	Kernel    string
	Arch      string
	Services  []Detection
	Available []Service
	Hostname  string
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func commandVersion(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func Detect() *Result {
	res := &Result{
		Hostname: commandVersion("hostname"),
	}

	if runtime.GOOS == "darwin" {
		res.OS = commandVersion("sw_vers", "-productName")
		res.Kernel = commandVersion("uname", "-r")
	} else {
		out := commandVersion("sh", "-c", "source /etc/os-release 2>/dev/null && echo $PRETTY_NAME")
		if out != "" {
			res.OS = out
		} else {
			res.OS = "Linux"
		}
		res.Kernel = commandVersion("uname", "-r")
	}

	res.Arch = commandVersion("uname", "-m")

	services := []struct {
		service  Service
		cmd      string
		verArgs  []string
	}{
		{ServiceMySQL, "mysqldump", []string{"--version"}},
		{ServicePostgres, "pg_dumpall", []string{"--version"}},
		{ServiceMongoDB, "mongodump", []string{"--version"}},
		{ServicePM2, "pm2", []string{"--version"}},
		{ServiceDocker, "docker", []string{"--version"}},
		{ServiceNginx, "nginx", []string{"-v"}},
		{ServiceApache, "apache2", []string{"-v"}},
		{ServiceCaddy, "caddy", []string{"version"}},
	}

	for _, s := range services {
		detect := Detection{
			Service:  s.service,
			Present:  commandExists(s.cmd),
			Commands: []string{s.cmd},
		}
		if detect.Present {
			detect.Version = commandVersion(s.cmd, s.verArgs...)
			res.Available = append(res.Available, s.service)
		}
		res.Services = append(res.Services, detect)
	}

	return res
}

func (r *Result) Has(service Service) bool {
	for _, s := range r.Available {
		if s == service {
			return true
		}
	}
	return false
}

