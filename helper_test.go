package influxdb

import (
	"fmt"
	"github.com/3th1nk/easygo/util/jsonUtil"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClient_CreateDatabase(t *testing.T) {
	_, err := testClient.CreateDatabase(testDB)
	assert.NoError(t, err)

	databases, err := testClient.ShowDatabases()
	assert.NoError(t, err)
	t.Log(databases)

	err = testClient.DropDatabase(testDB)
	assert.NoError(t, err)
}

func initTestDbRp() error {
	_, err := testClient.CreateDatabase(testDB)
	if err != nil {
		return err
	}

	_, err = testClient.CreateRetentionPolicy(testDB, testRP)
	return err
}

func TestClient_CreateRetentionPolicies(t *testing.T) {
	err := initTestDbRp()
	assert.NoError(t, err)

	data, err := testClient.ShowRetentionPolicies(testDB)
	fmt.Println(jsonUtil.MustMarshalToStringIndent(data))

	err = testClient.DropRetentionPolicy(testDB, testRP.Name)
	assert.NoError(t, err)
}
