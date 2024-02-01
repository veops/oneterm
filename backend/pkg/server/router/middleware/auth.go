// Package middleware
package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/veops/oneterm/pkg/conf"
	"github.com/veops/oneterm/pkg/logger"
	"github.com/veops/oneterm/pkg/server/auth/acl"
)

var (
	basicAuthDb = sync.Map{}
)

func init() {
	basicAuthDb.Store("admin", "admin")
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var err error
		var ok bool

		if conf.Cfg.Auth.Acl != nil && conf.Cfg.Auth.Acl.Url != "" {
			err, ok = authAcl(c)
		} else {
			// TODO: add your auth here
			ok = true
		}
		if !ok {
			logger.L.Warn(err.Error())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "authorized refused",
			})
			return
		}

		c.Next()
	}
}

func AuthToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Token") != conf.Cfg.SshServer.Xtoken {
			logger.L.Warn("invalid token", zap.String("X-Token", c.GetHeader("X-Token")))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "authorized refused",
			})
			return
		}

		c.Next()
	}
}

func authAcl(ctx *gin.Context) (error, bool) {
	session := &acl.Session{}

	sess, err := ctx.Cookie("session")
	if err == nil && sess != "" {
		s := acl.NewSignature(conf.Cfg.SecretKey, "cookie-session", "", "hmac", nil, nil)
		content, err := s.Unsign(sess)
		if err != nil {
			return err, false
		}

		err = json.Unmarshal(content, &session)
		if err != nil {
			return err, false
		}

		ctx.Set("session", session)
		return nil, true
	}
	return fmt.Errorf("no session"), false
}

//func authBasic(ctx *gin.Context) (error, bool) {
//	if user, password, ok := ctx.Request.BasicAuth(); ok {
//		if p, ok := basicAuthDb.Load(user); ok && p.(string) == password {
//			return nil, true
//		} else {
//			return fmt.Errorf("invalid user or password"), false
//		}
//	}
//	return fmt.Errorf("invalid user or password"), false
//}

//func authWithWhiteList(ip string) bool {
//	return lo.Contains(viper.GetStringSlice("gateway.whiteList"), ip)
//}
