package testsuite

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/gomodule/redigo/redis"
	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/gocommon/storage"
	"github.com/nyaruka/mailroom/core/models"
	"github.com/nyaruka/mailroom/runtime"
	"github.com/nyaruka/rp-indexer/v8/indexers"
	"github.com/olivere/elastic/v7"
	"github.com/sirupsen/logrus"
)

var _db *sqlx.DB

const elasticURL = "http://localhost:9200"
const elasticContactsIndex = "test_contacts"
const postgresContainerName = "textit-postgres-1"

const attachmentStorageDir = "_test_attachments_storage"
const sessionStorageDir = "_test_session_storage"
const logStorageDir = "_test_log_storage"

// Refresh is our type for the pieces of org assets we want fresh (not cached)
type ResetFlag int

// refresh bit masks
const (
	ResetAll     = ResetFlag(^0)
	ResetDB      = ResetFlag(1 << 1)
	ResetData    = ResetFlag(1 << 2)
	ResetRedis   = ResetFlag(1 << 3)
	ResetStorage = ResetFlag(1 << 4)
	ResetElastic = ResetFlag(1 << 5)
)

// Reset clears out both our database and redis DB
func Reset(what ResetFlag) {
	ctx := context.TODO()

	if what&ResetDB > 0 {
		resetDB()
	} else if what&ResetData > 0 {
		resetData()
	}
	if what&ResetRedis > 0 {
		resetRedis()
	}
	if what&ResetStorage > 0 {
		resetStorage()
	}
	if what&ResetElastic > 0 {
		resetElastic(ctx)
	}

	models.FlushCache()
}

// Runtime returns the various runtime things a test might need
func Runtime() (context.Context, *runtime.Runtime) {
	es, err := elastic.NewSimpleClient(elastic.SetURL(elasticURL), elastic.SetSniff(false))
	if err != nil {
		panic(err)
	}

	cfg := runtime.NewDefaultConfig()
	cfg.ElasticContactsIndex = elasticContactsIndex

	dbx := getDB()
	rt := &runtime.Runtime{
		DB:                dbx,
		ReadonlyDB:        dbx.DB,
		RP:                getRP(),
		ES:                es,
		AttachmentStorage: storage.NewFS(attachmentStorageDir, 0766),
		SessionStorage:    storage.NewFS(sessionStorageDir, 0766),
		LogStorage:        storage.NewFS(logStorageDir, 0766),
		Config:            cfg,
	}

	logrus.SetLevel(logrus.DebugLevel)

	return context.Background(), rt
}

// reindexes data changes to Elastic
func ReindexElastic(ctx context.Context) {
	db := getDB()
	es := getES()

	contactsIndexer := indexers.NewContactIndexer(elasticURL, elasticContactsIndex, 1, 1, 100)
	contactsIndexer.Index(db.DB, false, false)

	es.Refresh(elasticContactsIndex).Do(ctx)
}

// returns an open test database pool
func getDB() *sqlx.DB {
	if _db == nil {
		_db = sqlx.MustOpen("postgres", "postgres://mailroom_test:temba@localhost/mailroom_test?sslmode=disable&Timezone=UTC")

		// check if we have tables and if not load test database dump
		_, err := _db.Exec("SELECT * from orgs_org")
		if err != nil {
			loadTestDump()
			return getDB()
		}
	}
	return _db
}

// returns a redis pool to our test database
func getRP() *redis.Pool {
	return &redis.Pool{
		Dial: func() (redis.Conn, error) {
			conn, err := redis.Dial("tcp", "localhost:6379")
			if err != nil {
				return nil, err
			}
			_, err = conn.Do("SELECT", 0)
			return conn, err
		},
	}
}

// returns a redis connection, Close() should be called on it when done
func getRC() redis.Conn {
	conn, err := redis.Dial("tcp", "localhost:6379")
	noError(err)
	_, err = conn.Do("SELECT", 0)
	noError(err)
	return conn
}

// returns an Elastic client
func getES() *elastic.Client {
	es, err := elastic.NewSimpleClient(elastic.SetURL(elasticURL), elastic.SetSniff(false))
	noError(err)
	return es
}

// resets our database to our base state from our RapidPro dump
//
// mailroom_test.dump can be regenerated by running:
//
//	% python manage.py mailroom_db
//
// then copying the mailroom_test.dump file to your mailroom root directory
//
//	% cp mailroom_test.dump ../mailroom
func resetDB() {
	db := getDB()
	db.MustExec("DROP OWNED BY mailroom_test CASCADE")

	loadTestDump()
}

func loadTestDump() {
	dump, err := os.Open(absPath("./mailroom_test.dump"))
	must(err)
	defer dump.Close()

	cmd := exec.Command("docker", "exec", "-i", postgresContainerName, "pg_restore", "-d", "mailroom_test", "-U", "mailroom_test")
	cmd.Stdin = dump

	output, err := cmd.CombinedOutput()
	if err != nil {
		panic(fmt.Sprintf("error restoring database: %s: %s", err, string(output)))
	}

	// force re-connection
	if _db != nil {
		_db.Close()
		_db = nil
	}
}

// Converts a project root relative path to an absolute path usable in any test. This is needed because go tests
// are run with a working directory set to the current module being tested.
func absPath(p string) string {
	// start in working directory and go up until we are in a directory containing go.mod
	dir, _ := os.Getwd()
	for dir != "/" {
		dir = path.Dir(dir)
		if _, err := os.Stat(path.Join(dir, "go.mod")); err == nil {
			break
		}
	}
	return path.Join(dir, p)
}

// resets our redis database
func resetRedis() {
	rc, err := redis.Dial("tcp", "localhost:6379")
	if err != nil {
		panic(fmt.Sprintf("error connecting to redis db: %s", err.Error()))
	}
	rc.Do("SELECT", 0)
	_, err = rc.Do("FLUSHDB")
	if err != nil {
		panic(fmt.Sprintf("error flushing redis db: %s", err.Error()))
	}
}

// clears our storage for tests
func resetStorage() {
	must(os.RemoveAll(attachmentStorageDir))
	must(os.RemoveAll(sessionStorageDir))
	must(os.RemoveAll(logStorageDir))
}

// clears indexed data in Elastic
func resetElastic(ctx context.Context) {
	es := getES()

	exists, err := es.IndexExists(elasticContactsIndex).Do(ctx)
	noError(err)

	if exists {
		// get any indexes for the contacts alias
		ar, err := es.Aliases().Index(elasticContactsIndex).Do(ctx)
		noError(err)

		// and delete them
		for _, index := range ar.IndicesByAlias(elasticContactsIndex) {
			_, err := es.DeleteIndex(index).Do(ctx)
			noError(err)
		}
	}

	ReindexElastic(ctx)
}

var sqlResetTestData = `
UPDATE contacts_contact SET current_flow_id = NULL;

DELETE FROM tickets_ticketdailycount;
DELETE FROM tickets_ticketdailytiming;
DELETE FROM notifications_notification;
DELETE FROM notifications_incident;
DELETE FROM request_logs_httplog;
DELETE FROM tickets_ticketdailycount;
DELETE FROM tickets_ticketevent;
DELETE FROM tickets_ticket;
DELETE FROM triggers_trigger_contacts WHERE trigger_id >= 30000;
DELETE FROM triggers_trigger_groups WHERE trigger_id >= 30000;
DELETE FROM triggers_trigger_exclude_groups WHERE trigger_id >= 30000;
DELETE FROM triggers_trigger WHERE id >= 30000;
DELETE FROM channels_channelcount;
DELETE FROM msgs_msg;
DELETE FROM flows_flowrun;
DELETE FROM flows_flowpathcount;
DELETE FROM flows_flownodecount;
DELETE FROM flows_flowrunstatuscount;
DELETE FROM flows_flowcategorycount;
DELETE FROM flows_flowstart_contacts;
DELETE FROM flows_flowstart_groups;
DELETE FROM flows_flowstart;
DELETE FROM flows_flowsession;
DELETE FROM flows_flowrevision WHERE flow_id >= 30000;
DELETE FROM flows_flow WHERE id >= 30000;
DELETE FROM ivr_call;
DELETE FROM campaigns_eventfire;
DELETE FROM msgs_msg_labels;
DELETE FROM msgs_msg;
DELETE FROM msgs_broadcast_groups;
DELETE FROM msgs_broadcast_contacts;
DELETE FROM msgs_broadcastmsgcount;
DELETE FROM msgs_broadcast;
DELETE FROM schedules_schedule;
DELETE FROM campaigns_campaignevent WHERE id >= 30000;
DELETE FROM campaigns_campaign WHERE id >= 30000;
DELETE FROM contacts_contactimportbatch;
DELETE FROM contacts_contactimport;
DELETE FROM contacts_contacturn WHERE id >= 30000;
DELETE FROM contacts_contactgroup_contacts WHERE contact_id >= 30000 OR contactgroup_id >= 30000;
DELETE FROM contacts_contact WHERE id >= 30000;
DELETE FROM contacts_contactgroupcount WHERE group_id >= 30000;
DELETE FROM contacts_contactgroup WHERE id >= 30000;

ALTER SEQUENCE flows_flow_id_seq RESTART WITH 30000;
ALTER SEQUENCE tickets_ticket_id_seq RESTART WITH 1;
ALTER SEQUENCE msgs_msg_id_seq RESTART WITH 1;
ALTER SEQUENCE msgs_broadcast_id_seq RESTART WITH 1;
ALTER SEQUENCE flows_flowrun_id_seq RESTART WITH 1;
ALTER SEQUENCE flows_flowstart_id_seq RESTART WITH 1;
ALTER SEQUENCE flows_flowsession_id_seq RESTART WITH 1;
ALTER SEQUENCE contacts_contact_id_seq RESTART WITH 30000;
ALTER SEQUENCE contacts_contacturn_id_seq RESTART WITH 30000;
ALTER SEQUENCE contacts_contactgroup_id_seq RESTART WITH 30000;
ALTER SEQUENCE campaigns_campaign_id_seq RESTART WITH 30000;
ALTER SEQUENCE campaigns_campaignevent_id_seq RESTART WITH 30000;`

// removes contact data not in the test database dump. Note that this function can't
// undo changes made to the contact data in the test database dump.
func resetData() {
	db := getDB()
	db.MustExec(sqlResetTestData)

	// because groups have changed
	models.FlushCache()
}

// convenience way to call a func and panic if it errors, e.g. must(foo())
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// if just checking an error is nil noError(err) reads better than must(err)
var noError = must

func ReadFile(path string) []byte {
	d, err := os.ReadFile(path)
	noError(err)
	return d
}
