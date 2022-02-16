package scaleway

import (
	"github.com/Qovery/pleco/pkg/common"
	log "github.com/sirupsen/logrus"
)

func DeleteExpiredBuckets(sessions ScalewaySessions, options ScalewayOptions) {
	expiredBuckets := common.GetExpiredBuckets(sessions.Bucket, options.TagName, options.Region.String())

	count, start := common.ElemToDeleteFormattedInfos("expired bucket", len(expiredBuckets), string(options.Region))

	log.Debug(count)

	if options.DryRun || len(expiredBuckets) == 0 {
		return
	}

	log.Debug(start)

	for _, expiredBucket := range expiredBuckets {
		common.EmptyBucket(sessions.Bucket, expiredBucket.Name, expiredBucket.ObjectsInfos)
		common.DeleteBucket(sessions.Bucket, expiredBucket)
	}
}
