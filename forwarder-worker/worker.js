export default {
  async email(message, env, ctx) {
    // ================= 配置区域 =================
    const TARGET_URL = "https://bridge-server:8888/ingest"; 
    
    // 鉴权密钥
    // 建议在 Cloudflare 后台设置环境变量
    const AUTH_TOKEN = env.AUTH_TOKEN || "MySuperSecretToken123"; 
    // ===========================================

    console.log(`接收邮件: ${message.from} -> ${message.to}`);

    try {
      const response = await fetch(TARGET_URL, {
        method: "POST",
        headers: {
          "X-Auth-Token": AUTH_TOKEN,
          
          "X-Mail-From": message.from,
          "X-Mail-To": message.to,
          
          "Content-Type": "message/rfc822",
          "User-Agent": "CF-Worker-Forwarder/1.0"
        },

        body: message.raw, 
      });

      // 检查你的服务器是否成功接收
      if (!response.ok) {
        const errorText = await response.text();
        console.error(`Bridge-server 报错: ${response.status} - ${errorText}`);

        // 如果收件人邮箱没有在邮局注册，会报错
        // 这里可以使用 CF 的转发功能进行兜底，以做到 Catch-All 的效果
        // 使用前需要在 CF 验证邮箱
        /*
        message.setForward("man@example.com");
        */

        // 退信
        message.setReject(`服务器错误: ${response.status}`);
      } else {
        console.log("成功转发到 Bridge-server。");
      }

    } catch (e) {
      console.error(`网络或脚本错误: ${e.message}`);
      message.setReject("临时网络错误，请稍后重试。", 450); 
      /*
      message.setForward("man@example.com");
      */
    }
  }
};