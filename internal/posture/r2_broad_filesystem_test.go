package posture

import (
	"testing"

	"github.com/pilsu-choi/mcpgraph/internal/model"
)

func TestBroadFilesystem(t *testing.T) {
	tests := []struct {
		name string
		root string
		want int
	}{
		{"home", "/home/u", 1},
		{"home with trailing slash", "/home/u/", 1},
		{"filesystem root", "/", 1},
		{"project dir", "/home/u/code/app", 0},
		{"other user's home prefix", "/home/u2", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkFindings(t, BroadFilesystem{Home: "/home/u"}, fsServer(tt.root), tt.want, model.SeverityHigh)
		})
	}
}

func TestBroadFilesystemIgnoresOtherDataSources(t *testing.T) {
	b := &graphBuilder{}
	b.server("s", model.MCPServer{Name: "db"}).
		node(model.Node{ID: "ds", Kind: model.KindDataSource, DataSource: &model.DataSource{Kind: model.DataSourceDatabase, Identifier: "/"}}).
		edge("s", "ds", model.EdgeAccesses)
	checkFindings(t, BroadFilesystem{Home: "/home/u"}, &b.g, 0, model.SeverityHigh)
}
