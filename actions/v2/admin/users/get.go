package users

import (
	"net/http"

	"github.com/bitcoin-sv/spv-wallet/actions/v2/admin/internal/mapping"
	"github.com/bitcoin-sv/spv-wallet/engine/spverrors"
	"github.com/gin-gonic/gin"
)

// UserById returns a user by ID
func (s *APIAdminUsers) UserById(c *gin.Context, id string) {
	user, err := s.engine.UsersService().GetByID(c, id)
	if err != nil {
		spverrors.ErrorResponse(c, err, s.logger)
		return
	}

	c.JSON(http.StatusOK, mapping.UserToResponse(user))
}
