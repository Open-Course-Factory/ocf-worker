// internal/worker/worker_race_test.go - Test des race conditions

package worker

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// TestWorkerStateConsistency teste la cohérence de l'état du worker
func TestWorkerStateConsistency(t *testing.T) {
	// Créer un worker de test
	worker := &Worker{
		id:           1,
		status:       "idle",
		currentJobID: uuid.Nil,
	}

	// Nombre de goroutines concurrentes
	const numGoroutines = 100
	const numOperations = 1000

	var wg sync.WaitGroup
	inconsistencies := int64(0)

	// Lancer plusieurs goroutines qui modifient l'état
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				jobID := uuid.New()

				// Simuler le traitement d'un job
				worker.setState("busy", jobID)

				// Vérifier immédiatement la cohérence
				status, currentJobID := worker.getState()

				// Si le statut est "busy", l'ID du job ne devrait pas être nul
				if status == "busy" && currentJobID == uuid.Nil {
					inconsistencies++
				}

				// Simuler la fin du traitement
				worker.setState("idle", uuid.Nil)

				// Vérifier la cohérence
				status, currentJobID = worker.getState()

				// Si le statut est "idle", l'ID du job devrait être nul
				if status == "idle" && currentJobID != uuid.Nil {
					inconsistencies++
				}
			}
		}(i)
	}

	// Attendre que toutes les goroutines terminent
	wg.Wait()

	// Vérifier qu'il n'y a pas d'incohérences
	assert.Equal(t, int64(0), inconsistencies, "Detected state inconsistencies")
}

// TestWorkerStatisticsAtomic teste que les statistiques sont thread-safe
func TestWorkerStatisticsAtomic(t *testing.T) {
	worker := &Worker{
		id:           1,
		status:       "idle",
		currentJobID: uuid.Nil,
	}

	const numGoroutines = 50
	const numIncrements = 1000

	var wg sync.WaitGroup

	// Lancer plusieurs goroutines qui incrémentent les statistiques
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < numIncrements; j++ {
				atomic.AddInt64(&worker.jobsTotal, 1)

				// Simuler succès ou échec aléatoire
				if j%2 == 0 {
					atomic.AddInt64(&worker.jobsSuccess, 1)
				} else {
					atomic.AddInt64(&worker.jobsFailed, 1)
				}
			}
		}()
	}

	wg.Wait()

	// Vérifier les totaux
	stats := worker.GetStats()
	expectedTotal := int64(numGoroutines * numIncrements)
	expectedSuccess := expectedTotal / 2
	expectedFailed := expectedTotal / 2

	assert.Equal(t, expectedTotal, stats.JobsTotal)
	assert.Equal(t, expectedSuccess, stats.JobsSuccess)
	assert.Equal(t, expectedFailed, stats.JobsFailed)
}

// BenchmarkWorkerStateOperations benchmark les opérations d'état
func BenchmarkWorkerStateOperations(b *testing.B) {
	worker := &Worker{
		id:           1,
		status:       "idle",
		currentJobID: uuid.Nil,
	}

	jobID := uuid.New()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			worker.setState("busy", jobID)
			worker.getState()
			worker.setState("idle", uuid.Nil)
		}
	})
}
