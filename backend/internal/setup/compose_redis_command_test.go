package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRedisComposeUsesLiteralExecArguments(t *testing.T) {
	for _, name := range []string{"docker-compose.yml", "docker-compose.local.yml", "docker-compose.dev.yml"} {
		t.Run(name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", name))
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Services map[string]struct {
					Command yaml.Node `yaml:"command"`
				} `yaml:"services"`
			}
			if err := yaml.Unmarshal(body, &doc); err != nil {
				t.Fatal(err)
			}
			want := []string{"redis-server", "--save", "60", "1", "--appendonly", "yes", "--appendfsync", "everysec", "--requirepass", "${REDIS_PASSWORD:-}"}
			var got []string
			command := doc.Services["redis"].Command
			if err := command.Decode(&got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Redis must pass individual arguments without a shell: %#v", got)
			}
		})
	}
}
