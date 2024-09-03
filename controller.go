package main

import (
	"github.com/gin-gonic/gin"
	"github.com/we7coreteam/w7-rangine-go/v2/src/http/controller/validator/bind"
)

func Validate(ctx *gin.Context, requestData interface{}) bool {
	err := ctx.ShouldBindWith(requestData, bind.NewCompositeBind(ctx))
	if err != nil {
		return false
	}

	return true
}
