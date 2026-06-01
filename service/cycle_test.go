package service

import (
	"testing"
	"tutorgo/models"

	"github.com/stretchr/testify/assert"
)

func TestComputeCyclePositions(t *testing.T) {
	t.Run("assigns positions across two payment cycles", func(t *testing.T) {
		ranks := map[string]int{
			"l1": 1, "l2": 2, "l3": 3,
			"l4": 4, "l5": 5, "l6": 6, "l7": 7,
		}
		payments := []models.Payment{
			{LessonsCount: 3},
			{LessonsCount: 4},
		}

		result := computeCyclePositions(ranks, payments)

		assert.Equal(t, cycleInfo{Position: 1, Size: 3}, result["l1"])
		assert.Equal(t, cycleInfo{Position: 3, Size: 3}, result["l3"])
		assert.Equal(t, cycleInfo{Position: 1, Size: 4}, result["l4"])
		assert.Equal(t, cycleInfo{Position: 4, Size: 4}, result["l7"])
	})

	t.Run("lessons beyond total paid count have no entry", func(t *testing.T) {
		ranks := map[string]int{"l1": 1, "l2": 2, "l3": 3}
		payments := []models.Payment{{LessonsCount: 2}}

		result := computeCyclePositions(ranks, payments)

		assert.Contains(t, result, "l1")
		assert.Contains(t, result, "l2")
		assert.NotContains(t, result, "l3")
	})

	t.Run("no payments returns nil", func(t *testing.T) {
		ranks := map[string]int{"l1": 1}
		result := computeCyclePositions(ranks, nil)
		assert.Nil(t, result)
	})

	t.Run("empty ranks returns empty map", func(t *testing.T) {
		payments := []models.Payment{{LessonsCount: 8}}
		result := computeCyclePositions(map[string]int{}, payments)
		assert.Empty(t, result)
	})

	t.Run("single payment single lesson is both first and last", func(t *testing.T) {
		ranks := map[string]int{"l1": 1}
		payments := []models.Payment{{LessonsCount: 1}}

		result := computeCyclePositions(ranks, payments)

		assert.Equal(t, cycleInfo{Position: 1, Size: 1}, result["l1"])
	})
}
