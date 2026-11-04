//go:build integration

package quality

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestQualityExternalToolsPersistence(t *testing.T) {
	db := qualityTestDB(t)
	s := &Store{DB: db}
	ctx := context.Background()
	cfg, err := s.Settings(ctx)
	require.NoError(t, err)
	list, err := s.ExternalTools(ctx)
	require.NoError(t, err)
	require.Len(t, list.Items, 42)
	require.NoError(t, list.validate())
	require.Equal(t, "creative", list.Items[14].Kind)
	require.Equal(t, "图片站", list.Items[14].Notes)
	require.Equal(t, "unconfirmed", list.Items[14].Review)
	require.Equal(t, "各模型的运行状态", list.Items[8].Notes)
	require.Equal(t, "status", list.Items[8].Kind)
	require.Equal(t, "pricing", list.Items[10].Kind)
	require.Equal(t, "ranking", list.Items[5].Kind)
	require.Equal(t, "folkbench", list.Items[5].ID)
	stale := list
	list.Items = append(list.Items, ExternalTool{ID: "new-site", Name: "New site", URL: "https://example.com/", Kind: "ranking", Families: []string{"claude"}, Description: "Saved introduction"})
	saved, err := s.SaveExternalTools(ctx, list)
	require.NoError(t, err)
	_, err = s.SaveExternalTools(ctx, stale)
	require.True(t, isConflict(err))
	saved.Items[len(saved.Items)-1].Name = "Edited site"
	saved, err = s.SaveExternalTools(ctx, saved)
	require.NoError(t, err)
	migration, err := migrations.FS.ReadFile("254_quality_external_tools.sql")
	require.NoError(t, err)
	_, err = db.Exec(string(migration))
	require.NoError(t, err)
	loaded, err := s.ExternalTools(ctx)
	require.NoError(t, err)
	require.Equal(t, saved, loaded)
	saved.Items = nil
	_, err = s.SaveExternalTools(ctx, saved)
	require.NoError(t, err)
	loaded, err = s.ExternalTools(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded.Items)
	require.Empty(t, loaded.Items)
	// Administrator deletions must survive subsequent migration checks.
	for _, name := range []string{"254_quality_external_tools.sql", "255_quality_external_rankings.sql", "256_quality_external_resources.sql", "257_quality_business_resources.sql"} {
		migration, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		_, err = db.Exec(string(migration))
		require.NoError(t, err)
	}
	afterMigrations, err := s.ExternalTools(ctx)
	require.NoError(t, err)
	require.Equal(t, loaded, afterMigrations)
	afterCfg, err := s.Settings(ctx)
	require.NoError(t, err)
	require.Equal(t, cfg.Version, afterCfg.Version)
}

func TestQualityBusinessResourcesMigration(t *testing.T) {
	db := qualityTestDB(t)
	s := &Store{DB: db}
	ctx := context.Background()
	migration, err := migrations.FS.ReadFile("257_quality_business_resources.sql")
	require.NoError(t, err)
	for _, tc := range []struct {
		name  string
		count int
		want  int
		rich  bool
	}{
		{"deduplicates existing links", 1, 28, false},
		{"respects capacity", 99, 100, false},
		{"preserves a full list", 100, 100, false},
		{"respects content size", 63, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			list, err := s.ExternalTools(ctx)
			require.NoError(t, err)
			list.Items = []ExternalTool{{ID: "my-resend", Name: "My email provider", URL: "https://resend.com/signup/", Kind: "operations", Families: []string{}, Description: "Administrator introduction", Notes: "Preserve original note", Usage: "using"}}
			for i := 1; i < tc.count; i++ {
				list.Items = append(list.Items, ExternalTool{ID: fmt.Sprintf("custom-%d", i), Name: "Custom", URL: fmt.Sprintf("https://example.com/%d", i), Kind: "creative", Families: []string{}})
				if tc.rich {
					list.Items[i].Description = strings.Repeat("x", 2000)
					list.Items[i].Notes = strings.Repeat("y", 2000)
				}
			}
			saved, err := s.SaveExternalTools(ctx, list)
			require.NoError(t, err)
			_, err = db.Exec(`UPDATE quality_external_tools SET business_entries_initialized=FALSE WHERE id=TRUE`)
			require.NoError(t, err)
			_, err = db.Exec(string(migration))
			require.NoError(t, err)
			loaded, err := s.ExternalTools(ctx)
			require.NoError(t, err)
			if tc.rich {
				require.Less(t, len(loaded.Items), tc.count+27)
			} else {
				require.Len(t, loaded.Items, tc.want)
			}
			require.NoError(t, loaded.validate())
			require.Equal(t, saved.Items, loaded.Items[:tc.count])
			if tc.count == 100 {
				require.Equal(t, saved.Version, loaded.Version)
			} else {
				require.Equal(t, saved.Version+1, loaded.Version)
			}
			_, err = db.Exec(string(migration))
			require.NoError(t, err)
			again, err := s.ExternalTools(ctx)
			require.NoError(t, err)
			require.Equal(t, loaded, again)
		})
	}
}
