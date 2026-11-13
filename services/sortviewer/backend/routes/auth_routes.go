package routes

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"goklippy/services/sortviewer/backend/logic"
)

// health GET /api/health: up, and which build (mage deploy checks the commit).
func (a *API) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true, "commit": a.Commit})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login POST /api/auth/login {"email", "password"}: a viewer signs in.
func (a *API) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are needed"})
		return
	}
	token, err := a.Svc.Login(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	if err != nil {
		fail(c, err)
		return
	}
	a.setSessionCookie(c, token, int(logic.SessionTTL.Seconds()))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// logout POST /api/auth/logout
func (a *API) logout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookie); err == nil {
		_ = a.Svc.Logout(c.Request.Context(), token)
	}
	a.setSessionCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// me GET /api/auth/me: who's signed in, and whether admin sign-in is offered.
func (a *API) me(c *gin.Context) {
	out := gin.H{"signed_in": false, "admin_sign_in": a.Admin.Enabled() || a.Cfg.DevAdmin != ""}
	if sess, ok := session(c); ok {
		out["signed_in"] = true
		out["admin"] = sess.Admin()
		if sess.Admin() {
			out["name"], out["email"] = sess.AdminName, sess.AdminEmail
		} else {
			out["name"] = sess.ViewerName
		}
	}
	c.JSON(http.StatusOK, out)
}

// adminLoginStart GET /auth/login: off to the Gangway to sign in.
func (a *API) adminLoginStart(c *gin.Context) {
	if !a.Admin.Enabled() && a.Cfg.DevAdmin != "" {
		// Local development: no Gangway, sign straight in.
		a.finishAdminLogin(c, logic.AdminIdentity{Email: a.Cfg.DevAdmin, Name: "Dev admin"})
		return
	}
	to, err := a.Admin.Begin(c.Request.Context())
	if err != nil {
		slog.Warn("admin sign-in couldn't start", "error", err)
		c.Redirect(http.StatusFound, "/?signin=unavailable")
		return
	}
	c.Redirect(http.StatusFound, to)
}

// adminLoginCallback GET /auth/callback?code=&state=: back from the Gangway.
func (a *API) adminLoginCallback(c *gin.Context) {
	if c.Query("error") != "" {
		c.Redirect(http.StatusFound, "/?signin=cancelled")
		return
	}
	who, err := a.Admin.Finish(c.Request.Context(), c.Query("state"), c.Query("code"))
	if err != nil {
		slog.Warn("admin sign-in failed", "error", err)
		reason := "failed"
		if errors.Is(err, logic.ErrNotAdmin) {
			reason = "notadmin"
		}
		c.Redirect(http.StatusFound, "/?signin="+reason)
		return
	}
	a.finishAdminLogin(c, who)
}

func (a *API) finishAdminLogin(c *gin.Context, who logic.AdminIdentity) {
	token, err := a.Svc.AdminLogin(c.Request.Context(), who.Email, who.Name, who.Sid)
	if err != nil {
		fail(c, err)
		return
	}
	a.setSessionCookie(c, token, int(logic.SessionTTL.Seconds()))
	slog.Info("admin signed in", "email", who.Email)
	c.Redirect(http.StatusFound, "/admin")
}

// adminLogout GET /auth/logout: signs out here and of the Gangway portal.
func (a *API) adminLogout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookie); err == nil {
		_ = a.Svc.Logout(c.Request.Context(), token)
	}
	a.setSessionCookie(c, "", -1)
	if !a.Admin.Enabled() {
		c.Redirect(http.StatusFound, "/")
		return
	}
	c.Redirect(http.StatusFound, a.Admin.LogoutURL(c.Request.Context(), a.Cfg.AppURL+"/"))
}

// backchannelLogout POST /auth/backchannel-logout (logout_token=...): the Gangway
// ended a portal session, so the admin sessions from it end too.
func (a *API) backchannelLogout(c *gin.Context) {
	email, sid, err := a.Admin.BackchannelLogout(c.Request.Context(), c.PostForm("logout_token"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid logout token"})
		return
	}
	if err := a.Svc.EndAdminSessions(c.Request.Context(), email, sid); err != nil {
		fail(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)
}
