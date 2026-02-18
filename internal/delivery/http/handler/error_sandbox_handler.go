package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
)

// ErrorSandboxHandler exposes intentional error endpoints for learning purposes.
// Each route returns a realistic error payload so students can explore HTTP
// status codes without having to manufacture failure conditions themselves.
type ErrorSandboxHandler struct{}

func NewErrorSandboxHandler() *ErrorSandboxHandler {
	return &ErrorSandboxHandler{}
}

func errBody(c *fiber.Ctx, status int, code, title, detail, hint string) error {
	return c.Status(status).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"status": status,
			"code":   code,
			"title":  title,
			"detail": detail,
			"hint":   hint,
		},
		"docs": "https://freeapi.kidpech.app/playground.html",
	})
}

// 400 Bad Request
func (h *ErrorSandboxHandler) E400(c *fiber.Ctx) error {
	return errBody(c, 400,
		"VALIDATION_ERROR",
		"Bad Request",
		`field "price" must be a positive number, got: -9.99`,
		"ตรวจสอบ request body ให้ครบถ้วนและถูกต้องตาม schema",
	)
}

// 401 Unauthorized
func (h *ErrorSandboxHandler) E401(c *fiber.Ctx) error {
	return errBody(c, 401,
		"UNAUTHORIZED",
		"Unauthorized",
		"Authorization header is missing or token is expired",
		"ใส่ Bearer token ใน header: Authorization: Bearer <your-token>",
	)
}

// 403 Forbidden
func (h *ErrorSandboxHandler) E403(c *fiber.Ctx) error {
	return errBody(c, 403,
		"FORBIDDEN",
		"Forbidden",
		`role "user" is not allowed to access this resource (requires "admin")`,
		"ขอ token ด้วย role=admin ผ่าน POST /auth/login",
	)
}

// 404 Not Found
func (h *ErrorSandboxHandler) E404(c *fiber.Ctx) error {
	return errBody(c, 404,
		"NOT_FOUND",
		"Not Found",
		"product id=00000000-0000-0000-0000-000000000000 does not exist",
		"ตรวจสอบว่า ID ถูกต้อง และยังไม่ถูก soft-delete",
	)
}

// 405 Method Not Allowed
func (h *ErrorSandboxHandler) E405(c *fiber.Ctx) error {
	c.Set("Allow", "GET, POST")
	return errBody(c, 405,
		"METHOD_NOT_ALLOWED",
		"Method Not Allowed",
		"PATCH /api/v1/products is not supported — use PUT instead",
		"ดู Allow header เพื่อรู้ว่า method ไหนใช้ได้",
	)
}

// 409 Conflict
func (h *ErrorSandboxHandler) E409(c *fiber.Ctx) error {
	return errBody(c, 409,
		"CONFLICT",
		"Conflict",
		`SKU "CB-001" already exists for this user`,
		"ใช้ SKU ที่ไม่ซ้ำ หรือ PUT เพื่ออัปเดตของที่มีอยู่แล้ว",
	)
}

// 410 Gone
func (h *ErrorSandboxHandler) E410(c *fiber.Ctx) error {
	return errBody(c, 410,
		"GONE",
		"Gone",
		"product id=abc123 was permanently deleted and is no longer available",
		"ข้อมูลถูกลบถาวรแล้ว ไม่สามารถกู้คืนได้",
	)
}

// 422 Unprocessable Entity
func (h *ErrorSandboxHandler) E422(c *fiber.Ctx) error {
	return c.Status(422).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"status": 422,
			"code":   "UNPROCESSABLE_ENTITY",
			"title":  "Unprocessable Entity",
			"detail": "request body is syntactically valid JSON but semantically wrong",
			"hint":   "เช่น price=0, name=\" \" ผ่าน JSON parse แต่ logic บอกว่าไม่ถูกต้อง",
			"fields": []fiber.Map{
				{"field": "name", "error": "must not be blank"},
				{"field": "price", "error": "must be greater than 0"},
			},
		},
		"docs": "https://freeapi.kidpech.app/playground.html",
	})
}

// 429 Too Many Requests
func (h *ErrorSandboxHandler) E429(c *fiber.Ctx) error {
	reset := time.Now().Add(60 * time.Second).Unix()
	c.Set("X-RateLimit-Limit", "100")
	c.Set("X-RateLimit-Remaining", "0")
	c.Set("X-RateLimit-Reset", time.Unix(reset, 0).Format(time.RFC3339))
	c.Set("Retry-After", "60")
	return errBody(c, 429,
		"RATE_LIMIT_EXCEEDED",
		"Too Many Requests",
		"you have exceeded 100 requests per minute",
		"รอ 60 วินาทีแล้วลองใหม่ หรือดู Retry-After header",
	)
}

// 500 Internal Server Error
func (h *ErrorSandboxHandler) E500(c *fiber.Ctx) error {
	return errBody(c, 500,
		"INTERNAL_SERVER_ERROR",
		"Internal Server Error",
		"unexpected nil pointer dereference in product repository",
		"นี่คือ bug ของ server — client ทำอะไรไม่ได้มากนัก ลอง request ใหม่ หรือรอให้ dev แก้",
	)
}

// 502 Bad Gateway
func (h *ErrorSandboxHandler) E502(c *fiber.Ctx) error {
	return errBody(c, 502,
		"BAD_GATEWAY",
		"Bad Gateway",
		"upstream service (payment-svc) returned an invalid response",
		"เกิดจาก upstream service มีปัญหา — ลอง retry ด้วย exponential back-off",
	)
}

// 503 Service Unavailable
func (h *ErrorSandboxHandler) E503(c *fiber.Ctx) error {
	c.Set("Retry-After", "30")
	return errBody(c, 503,
		"SERVICE_UNAVAILABLE",
		"Service Unavailable",
		"redis connection pool exhausted — service is temporarily overloaded",
		"ดู Retry-After header และ implement circuit breaker ในฝั่ง client",
	)
}

// 504 Gateway Timeout
func (h *ErrorSandboxHandler) E504(c *fiber.Ctx) error {
	return errBody(c, 504,
		"GATEWAY_TIMEOUT",
		"Gateway Timeout",
		"upstream service did not respond within 10s deadline",
		"ตั้ง timeout ฝั่ง client ให้เหมาะสม และ handle ERROR นี้เพื่อ retry",
	)
}
