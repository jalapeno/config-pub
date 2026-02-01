package ingest

import (
	"context"
	"fmt"
	"io"
	"os"

	driver "github.com/arangodb/go-driver"
	"github.com/arangodb/go-driver/http"
)

const (
	defaultUserFile = "/credentials/.username"
	defaultPassFile = "/credentials/.password"
	maxCredential   = 256
)

type ArangoConfig struct {
	URL           string
	Database      string
	User          string
	Password      string
	UserFile      string
	PassFile      string
	IGPCollection string
	BGPCollection string
}

type ArangoClient struct {
	db            driver.Database
	igpCollection driver.Collection
	bgpCollection driver.Collection
}

func NewArangoClient(cfg ArangoConfig) (*ArangoClient, error) {
	if cfg.UserFile == "" {
		cfg.UserFile = defaultUserFile
	}
	if cfg.PassFile == "" {
		cfg.PassFile = defaultPassFile
	}
	if cfg.User == "" || cfg.Password == "" {
		user, pass, err := readCredentials(cfg.UserFile, cfg.PassFile)
		if err != nil {
			return nil, err
		}
		cfg.User = user
		cfg.Password = pass
	}
	conn, err := http.NewConnection(http.ConnectionConfig{Endpoints: []string{cfg.URL}})
	if err != nil {
		return nil, fmt.Errorf("arango connection: %w", err)
	}
	client, err := driver.NewClient(driver.ClientConfig{
		Connection:     conn,
		Authentication: driver.BasicAuthentication(cfg.User, cfg.Password),
	})
	if err != nil {
		return nil, fmt.Errorf("arango client: %w", err)
	}
	db, err := client.Database(context.Background(), cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("arango database: %w", err)
	}

	igp, err := db.Collection(context.Background(), cfg.IGPCollection)
	if err != nil {
		return nil, fmt.Errorf("igp collection: %w", err)
	}
	bgp, err := db.Collection(context.Background(), cfg.BGPCollection)
	if err != nil {
		return nil, fmt.Errorf("bgp collection: %w", err)
	}

	return &ArangoClient{
		db:            db,
		igpCollection: igp,
		bgpCollection: bgp,
	}, nil
}

func (c *ArangoClient) UpdateIGP(ctx context.Context, routerID, hostname string, update map[string]interface{}) (bool, error) {
	query := `
FOR n IN @@collection
	FILTER n.router_id == @router_id
	UPDATE n WITH @update IN @@collection OPTIONS { keepNull: false }
	RETURN NEW._key
`
	bindVars := map[string]interface{}{
		"@collection": c.igpCollection.Name(),
		"router_id":   routerID,
		"update":      update,
	}
	if routerID == "" && hostname != "" {
		query = `
FOR n IN @@collection
	FILTER n.name == @name
	UPDATE n WITH @update IN @@collection OPTIONS { keepNull: false }
	RETURN NEW._key
`
		bindVars = map[string]interface{}{
			"@collection": c.igpCollection.Name(),
			"name":        hostname,
			"update":      update,
		}
	}

	cursor, err := c.db.Query(ctx, query, bindVars)
	if err != nil {
		return false, err
	}
	defer cursor.Close()

	var key string
	_, err = cursor.ReadDocument(ctx, &key)
	if driver.IsNoMoreDocuments(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (c *ArangoClient) UpdateBGP(ctx context.Context, routerID string, asn int, update map[string]interface{}) (bool, error) {
	query := `
FOR n IN @@collection
	FILTER n.router_id == @router_id AND n.asn == @asn
	UPDATE n WITH @update IN @@collection OPTIONS { keepNull: false }
	RETURN NEW._key
`
	bindVars := map[string]interface{}{
		"@collection": c.bgpCollection.Name(),
		"router_id":   routerID,
		"asn":         asn,
		"update":      update,
	}

	cursor, err := c.db.Query(ctx, query, bindVars)
	if err != nil {
		return false, err
	}
	defer cursor.Close()

	var key string
	_, err = cursor.ReadDocument(ctx, &key)
	if driver.IsNoMoreDocuments(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func readCredentials(userFile, passFile string) (string, string, error) {
	user, err := readFile(userFile, maxCredential)
	if err != nil {
		return "", "", fmt.Errorf("read username: %w", err)
	}
	pass, err := readFile(passFile, maxCredential)
	if err != nil {
		return "", "", fmt.Errorf("read password: %w", err)
	}
	return user, pass, nil
}

func readFile(path string, max int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	if info.Size() > int64(max) {
		return "", fmt.Errorf("credential too long")
	}
	raw := make([]byte, info.Size())
	n, err := io.ReadFull(f, raw)
	if err != nil {
		return "", err
	}
	return string(raw[:n]), nil
}
