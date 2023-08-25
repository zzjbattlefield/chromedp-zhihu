package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/pkg/errors"
)

const cookieFile = "cookies.tmp"

func main() {
	//设置有头浏览器
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("headless", false))...)
	defer cancel()
	ctx, _ = chromedp.NewContext(ctx, chromedp.WithLogf(log.Printf))
	if err := zhihuLogin(ctx); err != nil {
		fmt.Printf("登录失败: %+v\n", err)
	}
	fmt.Println("登录成功")
}

func zhihuLogin(ctx context.Context) (err error) {
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	if err := chromedp.Run(ctx, modifyHeaders()); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, loadCookies()); err != nil {
		return err
	}
	var islogin bool
	if err := chromedp.Run(ctx, []chromedp.Action{
		chromedp.Navigate("https://www.zhihu.com/"),
		checkLoginStatus(&islogin),
	}...); err != nil {
		return err
	}
	if islogin {
		return nil
	}
	//继续执行登录和保存cookie操作
	if err := chromedp.Run(ctx, loginAndSave()); err != nil {
		return err
	}
	return
}

func modifyHeaders() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		headers := map[string]interface{}{
			"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/115.0.0.0 Safari/537.36",
			"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
			"Accept-Encoding": "gzip, deflate, br",
			"Accept-Language": "zh-CN,zh;q=0.9",
		}
		if err = network.SetExtraHTTPHeaders(headers).Do(ctx); err != nil {
			return errors.Wrap(err, "设置header失败")
		}
		return
	}
}

func loginAndSave() chromedp.Tasks {
	return chromedp.Tasks{
		chromedp.WaitVisible("#root > div > main > div > div > div > div > div.signQr-leftContainer > div.Qrcode-container.smallVersion > div.Qrcode-content > div.Qrcode-img > img", chromedp.ByQuery),
		getCode(),
		saveCookies(),
	}
}

// 获取二维码
func getCode() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		fmt.Println("获取二维码...")
		var code []byte
		if err := chromedp.Screenshot("#root > div > main > div > div > div > div > div.signQr-leftContainer > div.Qrcode-container.smallVersion > div.Qrcode-content > div.Qrcode-img > img", &code, chromedp.ByQuery).Do(ctx); err != nil {
			return errors.Wrap(err, "获取屏幕截图失败")
		}
		if err := os.WriteFile("code.png", code, 0755); err != nil {
			return errors.Wrap(err, "保存二维码失败")
		}
		fmt.Println("请扫码登录")
		return
	}
}

func checkLoginStatus(isLogin *bool) chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		var url string
		if err = chromedp.Evaluate(`window.location.href`, &url).Do(ctx); err != nil {
			return
		}
		if !strings.Contains(url, "signin?") {
			*isLogin = true
			log.Println("已经使用cookies登陆")
		}
		return
	}
}

func loadCookies() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		// 如果cookies临时文件不存在则直接跳过
		if _, _err := os.Stat(cookieFile); os.IsNotExist(_err) {
			return
		}

		// 如果存在则读取cookies的数据
		cookiesData, err := os.ReadFile(cookieFile)
		if err != nil {
			return errors.Wrap(err, "读取cookies文件失败")
		}

		// 反序列化
		cookiesParams := network.SetCookiesParams{}
		if err = cookiesParams.UnmarshalJSON(cookiesData); err != nil {
			return errors.Wrap(err, "反序列化cookies失败")
		}

		// 设置cookies
		return network.SetCookies(cookiesParams.Cookies).Do(ctx)
	}
}

func saveCookies() chromedp.ActionFunc {
	return func(ctx context.Context) (err error) {
		// 等待二维码登陆
		if err = chromedp.WaitVisible(`#root > div > main > div > div.Topstory-container`, chromedp.ByQuery).Do(ctx); err != nil {
			return
		}

		// cookies的获取对应是在devTools的network面板中
		// 1. 获取cookies
		cookies, err := network.GetCookies().Do(ctx)
		if err != nil {
			return
		}

		// 2. 序列化
		cookiesData, err := network.GetCookiesReturns{Cookies: cookies}.MarshalJSON()
		if err != nil {
			return
		}

		// 3. 存储到临时文件
		if err = os.WriteFile("cookies.tmp", cookiesData, 0755); err != nil {
			return
		}
		return
	}
}
