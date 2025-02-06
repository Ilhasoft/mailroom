package search

import (
	"bytes"
	"context"
	"fmt"

	"github.com/elastic/go-elasticsearch/v8/typedapi/types/enums/operationtype"
	"github.com/nyaruka/gocommon/elastic"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
)

// DeindexContactsByID de-indexes the contacts with the given IDs from Elastic
func DeindexContactsByID(ctx context.Context, rt *runtime.Runtime, orgID models.OrgID, contactIDs []models.ContactID) (int, error) {
	cmds := &bytes.Buffer{}
	for _, id := range contactIDs {
		cmds.Write(jsonx.MustMarshal(map[string]any{"delete": map[string]any{"_id": id.String()}}))
		cmds.WriteString("\n")
	}

	resp, err := rt.ES.Bulk().Index(rt.Config.ElasticContactsIndex).Routing(orgID.String()).Raw(bytes.NewReader(cmds.Bytes())).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("error deindexing deleted contacts from elastic: %w", err)
	}

	deleted := 0
	for _, r := range resp.Items {
		if r[operationtype.Delete].Status == 200 {
			deleted++
		}
	}

	return deleted, nil
}

// DeindexContactsByOrg de-indexes all contacts in the given org from Elastic
func DeindexContactsByOrg(ctx context.Context, rt *runtime.Runtime, orgID models.OrgID, limit int) (int, error) {
	src := map[string]any{
		"query":    elastic.Term("org_id", orgID),
		"max_docs": limit,
	}

	resp, err := rt.ES.DeleteByQuery(rt.Config.ElasticContactsIndex).Routing(orgID.String()).Raw(bytes.NewReader(jsonx.MustMarshal(src))).Do(ctx)
	if err != nil {
		return 0, fmt.Errorf("error deindexing contacts in org #%d from elastic: %w", orgID, err)
	}

	return int(*resp.Deleted), nil
}
