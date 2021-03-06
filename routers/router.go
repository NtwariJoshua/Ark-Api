// @APIVersion 1.0.0
// @Title beego Test API
// @Description beego has a very cool tools to autogenerate documents for your API
// @Contact astaxie@gmail.com
// @TermsOfServiceUrl http://beego.me/
// @License Apache 2.0
// @LicenseUrl http://www.apache.org/licenses/LICENSE-2.0.html
package routers

import (
	"ark-api/controllers"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	_"github.com/astaxie/beego/plugins/auth"
	"github.com/astaxie/beego/orm"
	"ark-api/models"
	"golang.org/x/crypto/bcrypt"
	_"ark-api/seeders"
	"github.com/astaxie/beego/plugins/cors"
	"ark-api/utils/data/plugins"
)

var AuthenticatedUser models.User

func init() {
	authenticator := plugins.NewBasicAuthenticator(SecretAuth, "Basic")
	ns := beego.NewNamespace("/api",
		//Cross-site reference
		beego.NSBefore(cors.Allow(&cors.Options{
			AllowAllOrigins: true,
			AllowMethods:     []string{"OPTIONS","GET","POST","PUT","DELETE"},
			AllowHeaders:     []string{"Authorization","Content-type","x-api-key"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
		})),

		beego.NSGet("/", func(ctx *context.Context) {
			ctx.Output.JSON(map[string]string{"Message":"Ark-Api V1"}, true, true)
		}),
		beego.NSNamespace("v1",
			beego.NSBefore(tenantCheckPoint,authenticator,userCheckPoint),
			//Admins & master tenant routes
				beego.NSNamespace("tenants",
					beego.NSBefore(masterTenantOnly),

					beego.NSRouter("/", &controllers.TenantsController{}, "get:Index"),
					beego.NSRouter("/", &controllers.TenantsController{}, "post:Store"),
					beego.NSRouter("/:id", &controllers.TenantsController{}, "put:Update"),
					beego.NSRouter("/:id", &controllers.TenantsController{}, "delete:Destroy"),

					beego.NSRouter("/:tenantId/master", &controllers.UsersController{}, "post:CreateTenantMasterUser"),
				),

				beego.NSNamespace("users",
					beego.NSBefore(adminsOnly),

					beego.NSRouter("/", &controllers.UsersController{}, "get:Index"),
					beego.NSRouter("/", &controllers.UsersController{}, "post:Store"),
					beego.NSRouter("/:id", &controllers.UsersController{}, "put:Update"),
					beego.NSRouter("/:id", &controllers.UsersController{}, "delete:Destroy"),
				),


			//Routes accessible to any authenticated user
				//Get the authenticated user
				beego.NSNamespace("auth",
					beego.NSRouter("/",&controllers.UsersController{},"get:Authenticate"),
				),
				// Product-categories
				beego.NSNamespace("product-categories",
					beego.NSRouter("/", &controllers.ProductCategoryController{}, "get:Index"),
					beego.NSRouter("/", &controllers.ProductCategoryController{}, "post:Store"),
					beego.NSRouter("/:id", &controllers.ProductCategoryController{}, "put:Update"),
					beego.NSRouter("/:id", &controllers.ProductCategoryController{}, "delete:Destroy"),
				),
				// Products
				beego.NSNamespace("products",
					beego.NSRouter("/", &controllers.ProductController{}, "get:Index"),
					beego.NSRouter("/", &controllers.ProductController{}, "post:Store"),
					beego.NSRouter("/:id", &controllers.ProductController{}, "put:Update"),
					beego.NSRouter("/:id", &controllers.ProductController{}, "delete:Destroy"),

					beego.NSRouter("/:productId/inventory",&controllers.InventoryController{},"get:Index"),

				),
				// Suppliers
				beego.NSNamespace("suppliers",
					beego.NSRouter("/", &controllers.SuppliersController{}, "get:Index"),
					beego.NSRouter("/", &controllers.SuppliersController{}, "post:Store"),
					beego.NSRouter("/:id", &controllers.SuppliersController{}, "put:Update"),
					beego.NSRouter("/:id", &controllers.SuppliersController{}, "delete:Destroy"),
				),
				// Purchase
				beego.NSNamespace("purchase",
					beego.NSRouter("/",&controllers.PurchaseController{},"get:Index"),
					beego.NSRouter("/",&controllers.PurchaseController{},"post:Store"),
				),
				// Sales
				beego.NSNamespace("sales",
					beego.NSRouter("/",&controllers.SalesController{},"post:NewSale"),
				),

			),
		)

	beego.AddNamespace(ns)

}

//Filters
func tenantCheckPoint(ctx *context.Context) {
	if ctx.Request.Header.Get("x-api-key") == "" {
		ctx.Output.Status = 401
		ctx.Output.JSON(map[string]string{"Error":"Access Denied, x-api-key header value is empty"}, true, true)
		return
	}
	tenant, ok := AuthenticateTenant(ctx.Request.Header.Get("x-api-key"))
	if !ok {
		ctx.Output.Status = 401
		ctx.Output.JSON(map[string]string{"Error":"Access Denied, x-api-key header value is not valid"}, true, true)
		return
	}
	ctx.Input.SetData("ActiveTenant", tenant)
}

func userCheckPoint(ctx *context.Context) {
	input := ctx.Input.Data()
	tenant := input["ActiveTenant"].(models.Tenant)
	user := AuthenticatedUser
	if user.Tenant.Id != tenant.Id {
		ctx.Output.Status = 401
		ctx.Output.JSON(map[string]string{"Error":"Access Denied"}, true, true)
		return
	}
	ctx.Input.SetData("AuthenticatedUser", AuthenticatedUser)
}

func masterTenantOnly(ctx *context.Context) {
	var input = ctx.Input.Data()
	tenant, _ := input["ActiveTenant"].(models.Tenant)
	if !tenant.IsMaster {
		ctx.Output.Status = 401
		ctx.Output.JSON(map[string]string{"Error":"Access Denied"}, true, true)
		return
	}
}

func adminsOnly(ctx *context.Context) {
	var input = ctx.Input.Data()
	user, _ := input["AuthenticatedUser"].(models.User)
	if !user.Role.IsAdmin() {
		ctx.Output.Status = 401
		ctx.Output.JSON(map[string]string{"Error":"Access Denied"}, true, true)
		return
	}

}






func SecretAuth(username, password string) bool {
	o := orm.NewOrm()
	err := o.QueryTable("user").Filter("email", username).RelatedSel("role", "tenant").One(&AuthenticatedUser)
	if err == orm.ErrNoRows {
		return false
	}
	return compareHashes(AuthenticatedUser.Password, password)
}

func AuthenticateTenant(api_key string) (models.Tenant, bool) {
	o := orm.NewOrm()
	tenant := models.Tenant{}
	err := o.QueryTable("tenant").Filter("api_key", api_key).One(&tenant)
	if err == orm.ErrNoRows {
		return tenant, false
	}
	return tenant, true
}

func compareHashes(val1, val2 string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(val1), []byte(val2))
	return err == nil
}


