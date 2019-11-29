package iam

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"git.topfreegames.com/libs-frameworks/remote-assets-api/app/helpers"
	"github.com/sirupsen/logrus"
)

type authStatus struct {
	statusCode  int
	accessToken string
	email       string
}

// WilliamPermissionBuilder ...
type WilliamPermissionBuilder func(projectName string) string

// WilliamAuthMiddlewareBuilder ...
func WilliamAuthMiddlewareBuilder(
	iamURL string, logger logrus.FieldLogger, permissionBuilder WilliamPermissionBuilder,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			projectName := q.Get("projectName")

			authorization := r.Header.Get("Authorization")
			authorizationComps := strings.Split(authorization, " ")

			// Check if the user sent an Authorization token. If it didn't, it means
			// that they are not logged into the IAM, and therefore we return a 401.
			if len(authorizationComps) != 2 {
				logger.Warn("Received request without Authorization header")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			permission := permissionBuilder(projectName)
			logger.Debugf("Permission %s was built\n", permission)

			status, err := getAuthStatus(iamURL, permission, authorization, logger)
			if err != nil {
				logger.Error("Failed to get auth status")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			if status.statusCode != http.StatusOK {
				logger.
					WithField("statusCode", status.statusCode).
					Warn("Authentication failed")
				helpers.WriteError(w, status.statusCode, "You have no permission to access this route")
				return
			}

			reqAuthValue := authorizationComps[1]
			if status.accessToken != "" && status.accessToken != reqAuthValue {
				w.Header().Set("x-access-token", status.accessToken)
			}

			if status.email != "" {
				w.Header().Set("x-email", status.email)
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getAuthStatus(
	iamURL, permission, authorization string, logger logrus.FieldLogger,
) (*authStatus, error) {
	client := &http.Client{}
	url := fmt.Sprintf(
		"%s/permissions/has?permission=%s",
		iamURL,
		url.QueryEscape(permission),
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authorization)

	fmt.Printf("Req %v\n", req)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return &authStatus{
		statusCode:  res.StatusCode,
		accessToken: res.Header.Get("x-access-token"),
		email:       res.Header.Get("x-email"),
	}, nil
}
