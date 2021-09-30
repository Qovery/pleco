package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	"github.com/scaleway/scaleway-sdk-go/api/rdb/v1"
	log "github.com/sirupsen/logrus"
	"time"
)

type ScalewayDB struct {
	ID           string
	Name         string
	CreationDate time.Time
	TTL          int64
	IsProtected  bool
}

func DeleteExpiredDatabases(sessions *ScalewaySessions, options *ScalewayOptions) {
	expiredDatabases, region := getExpiredDatabases(sessions.Database, options.TagName)

	count, start := common.ElemToDeleteFormattedInfos("expired database", len(expiredDatabases), region)

	log.Debug(count)

	if options.DryRun || len(expiredDatabases) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredDb := range expiredDatabases {
		deleteDB(sessions.Database, expiredDb)
	}
}

func getExpiredDatabases(dbAPI *rdb.API, tagName string) ([]ScalewayDB, string) {
	databases, region := listDatabases(dbAPI, tagName)

	expiredDbs := []ScalewayDB{}
	for _, db := range databases {
		if common.CheckIfExpired(db.CreationDate, db.TTL, "database"+db.Name) && !db.IsProtected {
			expiredDbs = append(expiredDbs, db)
		}
	}

	return expiredDbs, region
}

func listDatabases(dbAPI *rdb.API, tagName string) ([]ScalewayDB, string) {
	input := &rdb.ListInstancesRequest{}
	result, err := dbAPI.ListInstances(input)

	if err != nil {
		log.Errorf("Can't list databases for region %s: %s", input.Region, err.Error())
		return []ScalewayDB{}, input.Region.String()
	}

	databases := []ScalewayDB{}
	for _, db := range result.Instances {
		essentialTags := common.GetEssentialTags(db.Tags, tagName)
		creationDate, _ := time.Parse(time.RFC3339, db.CreatedAt.Format(time.RFC3339))

		databases = append(databases, ScalewayDB{
			ID:           db.ID,
			Name:         db.Name,
			CreationDate: creationDate,
			TTL:          essentialTags.TTL,
			IsProtected:  essentialTags.IsProtected,
		})
	}

	return databases, input.Region.String()
}

func deleteDB(dbAPI *rdb.API, db ScalewayDB) {
	_, err := dbAPI.DeleteInstance(
		&rdb.DeleteInstanceRequest{
			InstanceID: db.ID,
		})

	if err != nil {
		log.Errorf("Can't delete database %s: %s", db.Name, err.Error())
	}
}
