package httpServer

import (
	"github.com/get10xteam/sales-module-backend/app/opportunity"
	"github.com/get10xteam/sales-module-backend/app/user"
	"github.com/get10xteam/sales-module-backend/plumbings/oauth"
	"github.com/get10xteam/sales-module-backend/plumbings/storage"

	"github.com/gofiber/fiber/v2"
)

func apiRoutes(apiRouter fiber.Router) {
	authRoute := apiRouter.Group("auth")
	{ // auth

		// oath register
		authRoute.Post("register", user.UserSignUpHandler)      // tested
		authRoute.Get("register", user.UserSignUpVerifyHandler) // tested

		// oath login
		authRoute.Post("login", user.UserLoginHandler) // tested

		// get profile
		authRoute.Get("profile", user.MustAuthMiddleware, user.UserProfileHandler) // tested

		// edit profile
		authRoute.Put("profile",
			user.MustAuthMiddleware,
			storage.UploadHandlerFactory(&storage.UploadConfig{
				NonFormCallNext:  true,
				CallNextWhenDone: true,
				AllowedTypes:     []string{"image"},
				MaxSize:          1 * 1024 * 1024,
				PathPrefix:       "usrProfileImg",
				PathIncludeHash:  true,
			}),
			user.ChangeProfileHandler,
		) // tested for change name

		// oath reset password
		authRoute.Post("pwreset", user.UserResetPasswordStartHandler) // tested
		authRoute.Get("pwreset", user.UserResetPasswordSubmitHandler) // tested
		authRoute.Put("pwreset", user.UserResetPasswordSubmitHandler) // tested

		// oath google
		authRoute.Get("oauth/google", oauth.GoogleGetSignInUrlHandler)
		authRoute.Post("oauth/google", user.UserOauthLoginGoogleHandler)

		// oath microsoft
		authRoute.Get("oauth/microsoft", oauth.MicrosoftGetSignInUrlHandler)
		authRoute.Post("oauth/microsoft", user.UserOauthLoginMicrosoftHandler)

		// oath logout
		authRoute.Get("logout", user.UserLogoutHandler) // tested
	}
	users := apiRouter.Group("users")
	{ // user
		users.Get("dropdown", user.MustAuthMiddleware, user.UserDropdownHandler) // tested
	}
	opportunities := apiRouter.Group("opportunities")
	{ // opportunity
		opportunities.Post("", user.MustAuthMiddleware, opportunity.CreateOpportunityHandler) // tested
		opportunities.Get("", user.MustAuthMiddleware, opportunity.ListOpportunitiesHandler)  // tested
	}
}
