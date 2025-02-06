package search

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types"
	"github.com/nyaruka/gocommon/elastic"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/goflow/assets"
	"github.com/nyaruka/goflow/contactql"
	"github.com/nyaruka/goflow/contactql/es"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

// AssetMapper maps resolved assets in queries to how we identify them in ES which in the case
// of flows and groups is their ids. We can do this by just type cracking them to their models.
type AssetMapper struct{}

func (m *AssetMapper) Flow(f assets.Flow) int64 {
	return int64(f.(*models.Flow).ID())
}

func (m *AssetMapper) Group(g assets.Group) int64 {
	return int64(g.(*models.Group).ID())
}

var assetMapper = &AssetMapper{}

// BuildElasticQuery turns the passed in contact ql query into an elastic query
func BuildElasticQuery(oa *models.OrgAssets, group *models.Group, status models.ContactStatus, excludeIDs []models.ContactID, query *contactql.ContactQuery) elastic.Query {
	// filter by org and active contacts
	must := []elastic.Query{
		elastic.Term("org_id", oa.OrgID()),
		elastic.Term("is_active", true),
	}

	// and group if present
	if group != nil {
		must = append(must, elastic.Term("group_ids", group.ID()))
	}

	// and status if present
	if status != models.NilContactStatus {
		must = append(must, elastic.Term("status", status))
	}

	// and by user query if present
	if query != nil {
		must = append(must, es.ToElasticQuery(oa.Env(), assetMapper, query))
	}

	not := []elastic.Query{}

	// exclude ids if present
	if len(excludeIDs) > 0 {
		ids := make([]string, len(excludeIDs))
		for i := range excludeIDs {
			ids[i] = fmt.Sprintf("%d", excludeIDs[i])
		}
		not = append(not, elastic.Ids(ids...))
	}

	return elastic.Bool(must, not)
}

// GetContactTotal returns the total count of matching contacts for the given query
func GetContactTotal(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, group *models.Group, query string) (*contactql.ContactQuery, int64, error) {
	env := oa.Env()
	var parsed *contactql.ContactQuery
	var err error

	if rt.ES == nil {
		return nil, 0, fmt.Errorf("no elastic client available, check your configuration")
	}

	if query != "" {
		parsed, err = contactql.ParseQuery(env, query, oa.SessionAssets())
		if err != nil {
			return nil, 0, fmt.Errorf("error parsing query: %s: %w", query, err)
		}
	}

	// if group is a status group, Elastic won't know about it so search by status instead
	status := models.NilContactStatus
	if group != nil && !group.Visible() {
		status = models.ContactStatus(group.Type())
		group = nil
	}

	eq := BuildElasticQuery(oa, group, status, nil, parsed)
	src := map[string]any{"query": eq}

	count, err := rt.ES.Count().Index(rt.Config.ElasticContactsIndex).Routing(oa.OrgID().String()).Raw(bytes.NewReader(jsonx.MustMarshal(src))).Do(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("error performing count: %w", err)
	}

	return parsed, count.Count, nil
}

// GetContactIDsForQueryPage returns a page of contact ids for the given query and sort
func GetContactIDsForQueryPage(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, group *models.Group, excludeIDs []models.ContactID, query string, sort string, offset int, pageSize int) (*contactql.ContactQuery, []models.ContactID, int64, error) {
	env := oa.Env()
	index := rt.Config.ElasticContactsIndex
	start := time.Now()
	var parsed *contactql.ContactQuery
	var err error

	if rt.ES == nil {
		return nil, nil, 0, fmt.Errorf("no elastic client available, check your configuration")
	}

	if query != "" {
		parsed, err = contactql.ParseQuery(env, query, oa.SessionAssets())
		if err != nil {
			return nil, nil, 0, fmt.Errorf("error parsing query: %s: %w", query, err)
		}
	}

	// if group is a status group, Elastic won't know about it so search by status instead
	status := models.NilContactStatus
	if group != nil && !group.Visible() {
		status = models.ContactStatus(group.Type())
		group = nil
	}

	eq := BuildElasticQuery(oa, group, status, excludeIDs, parsed)

	fieldSort, err := es.ToElasticSort(sort, oa.SessionAssets())
	if err != nil {
		return nil, nil, 0, fmt.Errorf("error parsing sort: %w", err)
	}

	src := map[string]any{
		"_source":          false,
		"query":            eq,
		"sort":             []any{fieldSort},
		"from":             offset,
		"size":             pageSize,
		"track_total_hits": true,
	}

	results, err := rt.ES.Search().Index(index).Routing(oa.OrgID().String()).Raw(bytes.NewReader(jsonx.MustMarshal(src))).Do(ctx)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("error performing query: %w", err)
	}

	ids := make([]models.ContactID, 0, pageSize)
	ids = appendIDsFromHits(ids, results.Hits.Hits)

	slog.Debug("paged contact query complete", "org_id", oa.OrgID(), "query", query, "elapsed", time.Since(start), "page_count", len(ids), "total_count", results.Hits.Total.Value)

	return parsed, ids, results.Hits.Total.Value, nil
}

// GetContactIDsForQuery returns up to limit the contact ids that match the given query, sorted by id. Limit of -1 means return all.
func GetContactIDsForQuery(ctx context.Context, rt *runtime.Runtime, oa *models.OrgAssets, group *models.Group, status models.ContactStatus, query string, limit int) ([]models.ContactID, error) {
	env := oa.Env()
	index := rt.Config.ElasticContactsIndex
	var parsed *contactql.ContactQuery
	var err error

	if rt.ES == nil {
		return nil, fmt.Errorf("no elastic client available, check your configuration")
	}

	// turn into elastic query
	if query != "" {
		parsed, err = contactql.ParseQuery(env, query, oa.SessionAssets())
		if err != nil {
			return nil, fmt.Errorf("error parsing query: %s: %w", query, err)
		}
	}

	// if group is a status group, Elastic won't know about it so search by status instead
	if group != nil && !group.Visible() {
		status = models.ContactStatus(group.Type())
		group = nil
	}

	eq := BuildElasticQuery(oa, group, status, nil, parsed)
	sort := elastic.SortBy("id", true)
	ids := make([]models.ContactID, 0, 100)

	// if limit provided that can be done with single search, do that
	if limit >= 0 && limit <= 10_000 {
		src := map[string]any{
			"_source":          false,
			"query":            eq,
			"sort":             []any{sort},
			"from":             0,
			"size":             limit,
			"track_total_hits": false,
		}

		results, err := rt.ES.Search().Index(index).Routing(oa.OrgID().String()).Raw(bytes.NewReader(jsonx.MustMarshal(src))).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error searching ES index: %w", err)
		}
		return appendIDsFromHits(ids, results.Hits.Hits), nil
	}

	// for larger limits we need to take a point in time and iterate through multiple search requests using search_after
	pit, err := rt.ES.OpenPointInTime(index).Routing(oa.OrgID().String()).KeepAlive("1m").Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating ES point-in-time: %w", err)
	}

	src := map[string]any{
		"_source":          false,
		"query":            eq,
		"sort":             []any{sort},
		"pit":              map[string]any{"id": pit.Id, "keep_alive": "1m"},
		"size":             10_000,
		"track_total_hits": false,
	}

	for {
		results, err := rt.ES.Search().Raw(bytes.NewReader(jsonx.MustMarshal(src))).Do(ctx)
		if err != nil {
			return nil, fmt.Errorf("error searching ES index: %w", err)
		}

		if len(results.Hits.Hits) == 0 {
			break
		}

		ids = appendIDsFromHits(ids, results.Hits.Hits)

		lastHit := results.Hits.Hits[len(results.Hits.Hits)-1]
		src["search_after"] = lastHit.Sort
	}

	if _, err := rt.ES.ClosePointInTime().Id(pit.Id).Do(ctx); err != nil {
		return nil, fmt.Errorf("error closing ES point-in-time: %w", err)
	}

	return ids, nil
}

// utility to convert search hits to contact IDs and append them to the given slice
func appendIDsFromHits(ids []models.ContactID, hits []types.Hit) []models.ContactID {
	for _, hit := range hits {
		id, err := strconv.Atoi(*hit.Id_)
		if err == nil {
			ids = append(ids, models.ContactID(id))
		}
	}
	return ids
}
