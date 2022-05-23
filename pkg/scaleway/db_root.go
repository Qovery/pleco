package scaleway

import (
	"github.com/scaleway/scaleway-sdk-go/api/rdb/v1"
	log "github.com/sirupsen/logrus"
	"time"

	"github.com/Qovery/pleco/pkg/common"
)

type ScalewayDB struct {
	common.CloudProviderResource
	Name string
}

func DeleteExpiredDatabases(sessions ScalewaySessions, options ScalewayOptions) {
	expiredDatabases, _ := getExpiredDatabases(sessions.Database, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired database", len(expiredDatabases), options.Zone, true)

	log.Debug(count)

	if options.DryRun || len(expiredDatabases) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredDb := range expiredDatabases {
		deleteDB(sessions.Database, expiredDb)
	}
}

func getExpiredDatabases(dbAPI *rdb.API, options *ScalewayOptions) ([]ScalewayDB, string) {
	databases, region := listDatabases(dbAPI, options.TagName)

	expiredDbs := []ScalewayDB{}
	for _, db := range databases {
		if db.IsResourceExpired(options.TagValue) {
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
			CloudProviderResource: common.CloudProviderResource{
				Identifier:   db.ID,
				Description:  "Database: " + db.Name,
				CreationDate: creationDate,
				TTL:          essentialTags.TTL,
				Tag:          essentialTags.Tag,
				IsProtected:  essentialTags.IsProtected,
			},
			Name: db.Name,
		})
	}

	return databases, input.Region.String()
}

func deleteDB(dbAPI *rdb.API, db ScalewayDB) {
	_, err := dbAPI.DeleteInstance(
		&rdb.DeleteInstanceRequest{
			InstanceID: db.Identifier,
		})

	if err != nil {
		log.Errorf("Can't delete database %s: %s", db.Name, err.Error())
	}
}
