package controller

import (
	"strconv"

	"github.com/rakutentech/shibuya/shibuya/config"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func (c *Controller) deleteEngineHealthMetrics(collectionID string, planID string, engines int) {
	for i := 0; i < engines; i++ {
		engineID := strconv.Itoa(i)
		config.CpuGauge.Delete(prometheus.Labels{
			"collection_id": collectionID,
			"plan_id":       planID,
			"engine_no":     engineID,
		})
		config.MemGauge.Delete(prometheus.Labels{
			"collection_id": collectionID,
			"plan_id":       planID,
			"engine_no":     engineID,
		})
		log.Infof("Delete engine health metrics %s-%s-%s", collectionID, planID, engineID)
	}
}
