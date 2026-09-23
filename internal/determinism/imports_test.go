package determinism

import (
	"encoding/json"
	"io"
	"os/exec"
	"strings"
	"testing"
)

func TestSimulationHasNoExternalDependencies(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "-json", "github.com/karamble/dcrgaming-stakewars/pkg/sim/...")
	data, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	for {
		var pkg struct {
			ImportPath string
			Standard   bool
		}
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if !pkg.Standard && pkg.ImportPath != "github.com/karamble/dcrgaming-stakewars/pkg/sim" && !strings.HasPrefix(pkg.ImportPath, "github.com/karamble/dcrgaming-stakewars/pkg/sim/") {
			t.Errorf("external dependency in sim graph: %s", pkg.ImportPath)
		}
	}
}
