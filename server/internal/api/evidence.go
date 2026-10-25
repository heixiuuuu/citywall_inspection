package api

import (
	"citywall/server/internal/middleware"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func (h *ObservationHandler) Evidence(c *gin.Context) {
	actor, ok := middleware.GetActor(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "not authenticated")
		return
	}
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	result, err := h.service.Evidence(c.Request.Context(), actor, id)
	if err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"evidence": result})
}

func (h *ObservationHandler) RemoveImage(c *gin.Context) {
	actor, _ := middleware.GetActor(c)
	id, err := pathID(c)
	if err != nil {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid observation ID")
		return
	}
	imageID, err := strconv.ParseInt(c.Param("image_id"), 10, 64)
	if err != nil || imageID < 1 {
		writeError(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid image ID")
		return
	}
	if err := h.service.RemoveImage(c.Request.Context(), actor, id, imageID); err != nil {
		writeErr(c, err)
		return
	}
	writeData(c, http.StatusOK, gin.H{"removed": true})
}
