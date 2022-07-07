package do

import (
	"context"
	"time"

	"github.com/digitalocean/godo"
	log "github.com/sirupsen/logrus"

	"github.com/Qovery/pleco/pkg/common"
)

type DODB struct {
	common.CloudProviderResource
	Name string
}

func DeleteExpiredDatabases(sessions DOSessions, options DOOptions) {
	expiredDatabases := getExpiredDatabases(sessions.Client, &options)

	count, start := common.ElemToDeleteFormattedInfos("expired database", len(expiredDatabases), options.Region)

	log.Debug(count)

	if options.DryRun || len(expiredDatabases) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredDb := range expiredDatabases {
		deleteDB(sessions.Client, expiredDb)
	}
}

func getExpiredDatabases(client *godo.Client, options *DOOptions) []DODB {
	databases := listDatabases(client, options)

	expiredDbs := []DODB{}
	for _, db := range databases {
		if db.IsResourceExpired(options.TagValue, options.DisableTTLCheck) {
			expiredDbs = append(expiredDbs, db)
		}
	}

	return expiredDbs
}

func listDatabases(client *godo.Client, options *DOOptions) []DODB {
	result, _, err := client.Databases.List(context.TODO(), &godo.ListOptions{})

	if err != nil {
		log.Errorf("Can't list databases for region %s: %s", options.Region, err.Error())
		return []DODB{}
	}

	databases := []DODB{}
	for _, db := range result {
		essentialTags := common.GetEssentialTags(db.Tags, options.TagName)
		creationDate, _ := time.Parse(time.RFC3339, db.CreatedAt.Format(time.RFC3339))

		databases = append(databases, DODB{
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

	return databases
}

func deleteDB(client *godo.Client, db DODB) {
	_, err := client.Databases.Delete(context.TODO(), db.Identifier)

	if err != nil {
		log.Errorf("Can't delete database %s: %s", db.Name, err.Error())
	}
}
