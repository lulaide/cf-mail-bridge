package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var (
	// 目标 SMTP 服务器地址
	SMTP_HOST string

	// 鉴权 Token，必须与 Worker 一致
	AUTH_TOKEN string

	// 监听地址
	LISTEN_ADDR string

	// HELO 主机名
	HELO_HOST string
)

func main() {
	// 加载 .env 文件
	_ = godotenv.Load()

	// 初始化配置
	SMTP_HOST = getEnv("SMTP_HOST", "127.0.0.1:25")
	AUTH_TOKEN = getEnv("AUTH_TOKEN", "MySuperSecretToken123")
	LISTEN_ADDR = getEnv("LISTEN_ADDR", ":8888")
	HELO_HOST = getEnv("HELO_HOST", "127.0.0.1")

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	r.POST("/ingest", func(c *gin.Context) {
		// 安全校验
		requestToken := c.GetHeader("X-Auth-Token")
		if requestToken != AUTH_TOKEN {
			c.String(401, "Token 验证失败")
			return
		}

		// 获取信封信息
		mailFrom := c.GetHeader("X-Mail-From")
		mailTo := c.GetHeader("X-Mail-To")

		if mailFrom == "" || mailTo == "" {
			c.String(400, "缺少 X-Mail-From 或 X-Mail-To 头部信息")
			return
		}

		log.Printf("[INFO] 投递邮件: %s -> %s", mailFrom, mailTo)

		// 开始 SMTP 投递流程
		// 注意：我们将 c.Request.Body 传入，实现流式转发
		if err := forwardToSMTP(mailFrom, mailTo, c.Request.Body); err != nil {
			log.Printf("[ERROR] SMTP 转发失败: %v", err)
			// 返回 500 给 Cloudflare，这样 Cloudflare 会知道发送失败并生成退信通知
			c.String(500, fmt.Sprintf("SMTP 错误: %v", err))
			return
		}

		log.Printf("[SUCCESS] 邮件成功投递到目标 SMTP 服务器")
		c.String(200, "OK")
	})

	log.Printf("CF-MAIL-BRIDGE 监听地址: %s", LISTEN_ADDR)
	log.Printf("目标 SMTP: %s (HELO: %s)", SMTP_HOST, HELO_HOST)

	if err := r.Run(LISTEN_ADDR); err != nil {
		log.Fatal(err)
	}
}

// forwardToSMTP 负责建立 SMTP 连接并将 HTTP Body 流式传输进去
func forwardToSMTP(from, to string, bodyStream io.Reader) error {
	// 1. 建立 TCP 连接
	conn, err := net.Dial("tcp", SMTP_HOST)
	if err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}

	// 解析 Host 用于 Client 初始化
	host := SMTP_HOST
	if strings.Contains(host, ":") {
		host, _, _ = net.SplitHostPort(host)
	}

	// 2. 包装成 SMTP 客户端
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("客户端创建失败: %w", err)
	}
	defer c.Close()

	// 3. 发送 HELO
	if err := c.Hello(HELO_HOST); err != nil {
		return fmt.Errorf("HELO 命令失败: %w", err)
	}

	// 4. 设置发件人 (MAIL FROM)
	if err := c.Mail(from); err != nil {
		return fmt.Errorf("MAIL FROM 命令失败: %w", err)
	}

	// 5. 设置收件人 (RCPT TO)
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("RCPT TO 命令失败: %w", err)
	}

	// 6. 开始传输数据 (DATA)
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("DATA 命令失败: %w", err)
	}

	// 7. 流式复制数据 (零内存拷贝)
	// io.Copy 会自动处理缓冲区，将 HTTP 读取流对接给 SMTP 写入流
	if _, err := io.Copy(w, bodyStream); err != nil {
		return fmt.Errorf("流式复制失败: %w", err)
	}

	// 8. 结束传输 (.)
	if err := w.Close(); err != nil {
		return fmt.Errorf("数据关闭失败: %w", err)
	}

	// 9. 正常退出
	return c.Quit()
}

// 读取环境变量
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
