// Command extract writes one plugin's part of a running NetBox's OpenAPI
// schema: the paths under /api/plugins/<prefix>/ and every component they
// reference. The NetBox and plugin versions it came from are recorded under
// info.x-netbox-version and info.x-plugin-version.
//
//	go run ./tools/extract -prefix netbox-dns -status-key netbox_dns -out internal/plugins/dns/openapi.json
//
// NETBOX_SERVER_URL and NETBOX_API_TOKEN select the server.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
)

func main() {
	prefix := flag.String("prefix", "", "plugin API prefix, as in /api/plugins/<prefix>/")
	statusKey := flag.String("status-key", "", "plugin key under plugins in /api/status/")
	out := flag.String("out", "", "output file")
	flag.Parse()
	if *prefix == "" || *statusKey == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(os.Getenv("NETBOX_SERVER_URL"), os.Getenv("NETBOX_API_TOKEN"), *prefix, *statusKey, *out); err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(1)
	}
}

func run(server, token, prefix, statusKey, out string) error {
	if server == "" || token == "" {
		return fmt.Errorf("NETBOX_SERVER_URL and NETBOX_API_TOKEN must be set")
	}
	var status map[string]any
	if err := get(server, token, "/api/status/", &status); err != nil {
		return err
	}
	plugins, _ := status["plugins"].(map[string]any)
	pluginVersion, ok := plugins[statusKey].(string)
	if !ok {
		return fmt.Errorf("plugin %q is not installed on %s", statusKey, server)
	}

	var schema map[string]any
	if err := get(server, token, "/api/schema/?format=json", &schema); err != nil {
		return err
	}
	allPaths, _ := schema["paths"].(map[string]any)
	components, _ := schema["components"].(map[string]any)
	allSchemas, _ := components["schemas"].(map[string]any)

	pathPrefix := "/api/plugins/" + prefix + "/"
	paths := map[string]any{}
	for p, item := range allPaths {
		if strings.HasPrefix(p, pathPrefix) {
			paths[p] = item
		}
	}
	if len(paths) == 0 {
		return fmt.Errorf("no paths under %s", pathPrefix)
	}

	// Closure of the component schemas the plugin paths reference.
	keep := map[string]any{}
	queue := refs(paths)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if _, done := keep[name]; done {
			continue
		}
		s, ok := allSchemas[name]
		if !ok {
			return fmt.Errorf("schema %q referenced but not defined", name)
		}
		keep[name] = s
		queue = append(queue, refs(s)...)
	}

	doc := map[string]any{
		"openapi": schema["openapi"],
		"info": map[string]any{
			"title":            "NetBox plugin " + statusKey,
			"version":          pluginVersion,
			"x-netbox-version": status["netbox-version"],
			"x-plugin-version": pluginVersion,
		},
		"paths":      paths,
		"components": map[string]any{"schemas": keep},
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %d paths, %d schemas (NetBox %v, %s %s)\n", out, len(paths), len(keep), status["netbox-version"], statusKey, pluginVersion)
	return nil
}

// refs returns the component names referenced anywhere in v, sorted.
func refs(v any) []string {
	seen := map[string]bool{}
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if r, ok := t["$ref"].(string); ok {
				if name, ok := strings.CutPrefix(r, "#/components/schemas/"); ok {
					seen[name] = true
				}
			}
			for _, c := range t {
				walk(c)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	walk(v)
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func get(server, token, path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(server, "/")+path, nil)
	if err != nil {
		return err
	}
	scheme := "Token "
	if strings.HasPrefix(token, "nbt_") {
		scheme = "Bearer "
	}
	req.Header.Set("Authorization", scheme+token)
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s: %s", path, resp.Status, body)
	}
	return json.Unmarshal(body, out)
}
