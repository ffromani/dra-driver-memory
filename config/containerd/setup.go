package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/pelletier/go-toml/v2"
)

func main() {
	exePath := os.Args[0]
	args := os.Args[1:]
	if len(args) != 1 {
		usage(exePath)
		os.Exit(1)
	}
	arg := args[0] // shortcut
	if arg == "-h" || arg == "--help" {
		usage(exePath)
		os.Exit(0)
	}

	var err error
	if arg == "-" {
		err = handleStream(os.Stdin, os.Stdout)
	} else {
		err = handleInplace(arg)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error processing %q: %v\n", arg, err)
		os.Exit(127)
	}
}

func usage(exePath string) {
	fmt.Fprintf(os.Stderr, "usage: %s /path/to/conf.toml\n", exePath)
	fmt.Fprintf(os.Stderr, "set up the containerd conf at \"/path/to/conf.toml\"\n")
	fmt.Fprintf(os.Stderr, "use \"-\" to read from stdin and write from stdout\n")
}

func handleStream(src io.Reader, dst io.Writer) error {
	data, err := io.ReadAll(src)
	if err != nil {
		return err
	}

	var conf map[string]any
	err = toml.Unmarshal(data, &conf)
	if err != nil {
		return err
	}

	process(conf)

	b, err := toml.Marshal(conf)
	if err != nil {
		return err
	}

	_, err = dst.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func handleInplace(confPath string) error {
	finfo, err := os.Lstat(confPath)
	if err != nil {
		return err
	}
	inData, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	inBuf := bytes.NewBuffer(inData)
	outBuf := new(bytes.Buffer)
	err = handleStream(inBuf, outBuf)
	if err != nil {
		return err
	}
	return os.WriteFile(confPath, outBuf.Bytes(), finfo.Mode())
}

func process(conf map[string]any) {
	plugins, ok := getMap(conf, "plugins")
	if !ok {
		return
	}

	processNRI(plugins)

	cri, ok := getMap(plugins, "io.containerd.grpc.v1.cri")
	if !ok {
		return
	}

	processCDI(cri)
	processHugepages(cri)
}

func processNRI(plugins map[string]any) {
	plugins["io.containerd.nri.v1.nri"] = map[string]any{
		"disable":                     false,
		"disable_connections":         false,
		"plugin_config_path":          "/etc/nri/conf.d",
		"plugin_path":                 "/opt/nri/plugins",
		"plugin_registration_timeout": "5s",
		"plugin_request_timeout":      "5s",
		"socket_path":                 "/var/run/nri/nri.sock",
	}
}

func processCDI(cri map[string]any) {
	cri["enable_cdi"] = true
	cri["cdi_spec_dirs"] = []string{"/etc/cdi", "/var/run/cdi"}
}

func processHugepages(cri map[string]any) {
	cri["tolerate_missing_hugepages_controller"] = false
	cri["disable_hugetlb_controller"] = false
}

func getMap(node map[string]any, key string) (map[string]any, bool) {
	subNode, ok := node[key]
	if !ok {
		return nil, false
	}
	subMap, ok := subNode.(map[string]any)
	if !ok {
		return nil, false
	}
	return subMap, true
}
