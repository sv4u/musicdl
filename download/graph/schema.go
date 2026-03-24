package graph

import (
	"context"
	"log"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// initSchema creates constraints and indexes for the graph.
func (c *Client) initSchema(ctx context.Context) error {
	constraints := []string{
		"CREATE CONSTRAINT artist_spotify_id IF NOT EXISTS FOR (a:Artist) REQUIRE a.spotify_id IS UNIQUE",
		"CREATE CONSTRAINT album_spotify_id IF NOT EXISTS FOR (a:Album) REQUIRE a.spotify_id IS UNIQUE",
		"CREATE CONSTRAINT track_item_id IF NOT EXISTS FOR (t:Track) REQUIRE t.item_id IS UNIQUE",
		"CREATE CONSTRAINT playlist_spotify_id IF NOT EXISTS FOR (p:Playlist) REQUIRE p.spotify_id IS UNIQUE",
		"CREATE CONSTRAINT m3u_file_path IF NOT EXISTS FOR (m:M3UFile) REQUIRE m.file_path IS UNIQUE",
		"CREATE CONSTRAINT run_id IF NOT EXISTS FOR (r:DownloadRun) REQUIRE r.run_id IS UNIQUE",
	}

	indexes := []string{
		"CREATE INDEX track_name IF NOT EXISTS FOR (t:Track) ON (t.name)",
		"CREATE INDEX track_spotify_id IF NOT EXISTS FOR (t:Track) ON (t.spotify_id)",
		"CREATE INDEX track_file_path IF NOT EXISTS FOR (t:Track) ON (t.file_path)",
		"CREATE INDEX track_status IF NOT EXISTS FOR (t:Track) ON (t.status)",
		"CREATE INDEX artist_name IF NOT EXISTS FOR (a:Artist) ON (a.name)",
		"CREATE INDEX album_name IF NOT EXISTS FOR (a:Album) ON (a.name)",
		"CREATE INDEX playlist_name IF NOT EXISTS FOR (p:Playlist) ON (p.name)",
		"CREATE INDEX m3u_name IF NOT EXISTS FOR (m:M3UFile) ON (m.name)",
		"CREATE INDEX run_started IF NOT EXISTS FOR (r:DownloadRun) ON (r.started_at)",
	}

	session := c.session(ctx)
	defer func() { _ = session.Close(ctx) }()

	for _, stmt := range constraints {
		if _, err := session.Run(ctx, stmt, nil); err != nil {
			log.Printf("WARN: graph_schema constraint skipped: %v", err)
		}
	}

	for _, stmt := range indexes {
		if _, err := session.Run(ctx, stmt, nil); err != nil {
			log.Printf("WARN: graph_schema index skipped: %v", err)
		}
	}

	log.Printf("INFO: graph_schema initialized (%d constraints, %d indexes)", len(constraints), len(indexes))
	return nil
}

// fullTextIndexes creates full-text search indexes. Called separately because
// they require CALL db.index.fulltext.createNodeIndex which may not be
// available in all Neo4j editions.
func (c *Client) ensureFullTextIndex(ctx context.Context) {
	stmt := `CALL db.index.fulltext.createNodeIndex(
		"musicdl_search",
		["Track", "Artist", "Album", "Playlist", "M3UFile"],
		["name", "file_path", "spotify_url", "error"],
		{analyzer: "standard-no-stop-words"}
	)`

	session := c.session(ctx)
	defer func() { _ = session.Close(ctx) }()

	if _, err := session.Run(ctx, stmt, nil); err != nil {
		log.Printf("INFO: graph_schema full-text index creation skipped (may already exist): %v", err)
	}
}

// DropAll removes all nodes and relationships. Use for testing or reset.
func (c *Client) DropAll(ctx context.Context) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, "MATCH (n) DETACH DELETE n", nil)
		return nil, err
	})
}
