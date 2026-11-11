package usagecontent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestSQLContentBatchKeepsBillingAndClientIdentitiesSeparate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := &SQLStore{DB: db}
	now := time.Now().UTC()
	for _, billingID := range []string{"client:billed", ""} {
		r := Record{APIKeyID: 4, Reference: Reference{CaptureID: "00000000-0000-0000-0000-000000000001", RequestID: billingID, ClientRequestID: "internal-client-id", CreatedAt: now, ExpiresAt: now.Add(Retention)}, RequestGZIP: []byte("gzip-input"), ResponseGZIP: []byte("gzip-output")}
		meta, err := json.Marshal(r.Metadata)
		require.NoError(t, err)
		size := len(meta) + len(r.RequestGZIP) + len(r.ResponseGZIP) + 512
		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT used_bytes FROM usage_content_budget WHERE id=1 FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"used_bytes"}).AddRow(0))
		mock.ExpectQuery(`INSERT INTO usage_request_contents`).WithArgs(r.CaptureID, billingID, int64(4), r.ClientRequestID, now, now.Add(Retention), sqlmock.AnyArg(), r.RequestGZIP, r.ResponseGZIP, 1, size).WillReturnRows(sqlmock.NewRows([]string{"storage_bytes"}).AddRow(size))
		mock.ExpectExec(`UPDATE usage_content_budget SET used_bytes=used_bytes\+\$1 WHERE id=1`).WithArgs(int64(size)).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		require.NoError(t, s.SaveBatch(context.Background(), []Record{r}))
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLContentReadsAreKeyScopedAndFilterExpiryWithoutLoadingBodiesInReferences(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := &SQLStore{DB: db}
	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT capture_id,billing_request_id,client_request_id,created_at,expires_at,metadata FROM usage_request_contents WHERE api_key_id=\$1 AND expires_at > NOW\(\) AND billing_request_id=\$2`).WithArgs(int64(2), "billing-id").WillReturnRows(sqlmock.NewRows([]string{"capture_id", "billing_request_id", "client_request_id", "created_at", "expires_at", "metadata"}).AddRow("capture", "billing-id", "client-id", now, now.Add(Retention), `{}`))
	refs, err := s.References(context.Background(), Query{APIKeyID: 2, RequestID: "billing-id"})
	require.NoError(t, err)
	require.Len(t, refs, 1)
	mock.ExpectQuery(`SELECT .*request_body,response_body FROM usage_request_contents WHERE api_key_id=\$1 AND expires_at > NOW\(\) AND client_request_id=\$2 AND capture_id=\$3::uuid`).WithArgs(int64(2), "client-id", "capture").WillReturnRows(sqlmock.NewRows([]string{"capture_id", "billing_request_id", "client_request_id", "created_at", "expires_at", "metadata", "request_body", "response_body"}).AddRow("capture", "billing-id", "client-id", now, now.Add(Retention), `{}`, []byte("a"), []byte("b")))
	r, err := s.Content(context.Background(), Query{APIKeyID: 2, ClientRequestID: "client-id", CaptureID: "capture"})
	require.NoError(t, err)
	require.Equal(t, "a", string(r.RequestGZIP))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLContentCleanupIsIndexedBatchedAndSkipsOtherInstancesLocks(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := &SQLStore{DB: db}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT used_bytes FROM usage_content_budget WHERE id=1 FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"used_bytes"}).AddRow(5000))
	mock.ExpectQuery(`WITH deleted AS \(DELETE FROM usage_request_contents WHERE .*expires_at <= NOW\(\) ORDER BY expires_at LIMIT \$1 FOR UPDATE SKIP LOCKED`).WithArgs(1000).WillReturnRows(sqlmock.NewRows([]string{"count", "freed"}).AddRow(21, 1200))
	mock.ExpectExec(`UPDATE usage_content_budget SET used_bytes=GREATEST`).WithArgs(int64(1200)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	n, err := s.DeleteExpired(context.Background(), 1000)
	require.NoError(t, err)
	require.EqualValues(t, 21, n)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLContentStorageBudgetFailsOpenWithoutInsertingOrOvercounting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := &SQLStore{DB: db, MaxStorageBytes: 1}
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT used_bytes FROM usage_content_budget WHERE id=1 FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"used_bytes"}).AddRow(0))
	mock.ExpectRollback()
	err = s.SaveBatch(context.Background(), []Record{{Reference: Reference{ExpiresAt: time.Now().Add(time.Hour)}}})
	require.ErrorIs(t, err, ErrStorageLimit)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSQLContentStartupReconcilesBudgetAfterPayloadFreeBackupRestore(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := &SQLStore{DB: db}
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO usage_content_budget`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT used_bytes FROM usage_content_budget WHERE id=1 FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"used_bytes"}).AddRow(999999))
	mock.ExpectExec(`UPDATE usage_content_budget SET used_bytes=\(SELECT COALESCE\(SUM\(storage_bytes\),0\) FROM usage_request_contents\) WHERE id=1`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	require.NoError(t, s.Initialize(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}
