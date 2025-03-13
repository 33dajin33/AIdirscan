package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
)

// 扫描结果结构体
type ScanResult struct {
	URL        string
	StatusCode int
	Time       time.Duration
}

// 从字典文件读取扫描路径
func loadDictionary() ([]string, error) {
	// 获取当前目录下的dicc/dicc.txt文件
	dictPath := filepath.Join("dicc", "dicc.txt")

	// 检查文件是否存在
	if _, err := os.Stat(dictPath); os.IsNotExist(err) {
		// 如果文件不存在，创建目录和文件
		if err := os.MkdirAll("dicc", 0755); err != nil {
			return nil, fmt.Errorf("创建目录失败: %v", err)
		}

		// 创建默认的字典文件
		defaultPaths := []string{
			"/",
			"/admin",
			"/login",
			"/wp-admin",
			"/phpinfo.php",
			"/robots.txt",
			"/sitemap.xml",
			"/.git",
			"/.env",
			"/api",
			"/backup",
			"/config",
			"/data",
			"/db",
			"/docs",
			"/images",
			"/includes",
			"/install",
			"/logs",
			"/media",
			"/php",
			"/scripts",
			"/src",
			"/static",
			"/templates",
			"/tmp",
			"/uploads",
			"/vendor",
		}

		file, err := os.Create(dictPath)
		if err != nil {
			return nil, fmt.Errorf("创建字典文件失败: %v", err)
		}
		defer file.Close()

		// 写入默认路径
		for _, path := range defaultPaths {
			fmt.Fprintln(file, path)
		}

		fmt.Printf("已创建默认字典文件: %s\n", dictPath)
		return defaultPaths, nil
	}

	// 读取字典文件
	file, err := os.Open(dictPath)
	if err != nil {
		return nil, fmt.Errorf("打开字典文件失败: %v", err)
	}
	defer file.Close()

	var paths []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := strings.TrimSpace(scanner.Text())
		if path != "" && !strings.HasPrefix(path, "#") {
			// 确保路径以/开头
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			paths = append(paths, path)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取字典文件失败: %v", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("字典文件为空")
	}

	return paths, nil
}

// 从URL生成文件名
func generateFileName(inputURL string) (string, error) {
	// 解析URL
	parsedURL, err := url.Parse(inputURL)
	if err != nil {
		return "", err
	}

	// 获取主机部分（包含端口）
	host := parsedURL.Host

	// 替换所有的.为_
	fileName := strings.ReplaceAll(host, ".", "_")
	// 替换:为_
	fileName = strings.ReplaceAll(fileName, ":", "_")

	return fileName, nil
}

// 保存结果到文件
func saveResults(results []ScanResult, fileName string) error {
	// 创建result目录
	resultDir := "result"
	if err := os.MkdirAll(resultDir, 0755); err != nil {
		return fmt.Errorf("创建结果目录失败: %v", err)
	}

	// 创建结果文件
	filePath := filepath.Join(resultDir, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建结果文件失败: %v", err)
	}
	defer file.Close()

	// 写入表头
	fmt.Fprintf(file, "%-50s %-15s %-15s\n", "URL", "状态码", "响应时间")
	fmt.Fprintln(file, strings.Repeat("-", 80))

	// 写入结果
	for _, result := range results {
		fmt.Fprintf(file, "%-50s %-15d %-15s\n",
			result.URL,
			result.StatusCode,
			result.Time.Round(time.Millisecond))
	}

	return nil
}

func main() {
	// 定义命令行参数
	var (
		url         string
		urlFile     string
		statusCodes string
		ignoreCodes string
		threads     int
		help        bool
	)

	// 设置命令行参数
	flag.StringVar(&url, "u", "", "目标URL (例如: http://example.com)")
	flag.StringVar(&urlFile, "f", "", "包含多个URL的文件路径，每行一个URL")
	flag.StringVar(&statusCodes, "s", "", "要显示的响应码，用逗号分隔 (例如: 200,301,403,404)")
	flag.StringVar(&ignoreCodes, "i", "", "要忽略的响应码，用逗号分隔 (例如: 404,503)")
	flag.IntVar(&threads, "t", 10, "并发线程数 (默认: 10)")
	flag.BoolVar(&help, "h", false, "显示帮助信息")

	// 自定义usage信息
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "目录扫描工具 v1.0\n\n")
		fmt.Fprintf(os.Stderr, "用法:\n")
		fmt.Fprintf(os.Stderr, "  %s (-u URL | -f URL_FILE) [-s STATUS_CODES] [-i IGNORE_CODES] [-t THREADS]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "参数:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n说明:\n")
		fmt.Fprintf(os.Stderr, "  扫描路径从 ./dicc/dicc.txt 文件中读取，每行一个路径\n")
		fmt.Fprintf(os.Stderr, "\n示例:\n")
		fmt.Fprintf(os.Stderr, "  %s -u http://example.com -s 200,301,403 -i 404,503 -t 20\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s -f urls.txt -s 200,301,403 -i 404 -t 50\n", os.Args[0])
	}

	// 解析命令行参数
	flag.Parse()

	// 如果指定了-h或没有指定必需参数，显示帮助信息
	if help || (url == "" && urlFile == "") {
		flag.Usage()
		os.Exit(0)
	}

	// 加载扫描路径字典
	paths, err := loadDictionary()
	if err != nil {
		fmt.Printf("加载字典失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已加载 %d 个扫描路径\n", len(paths))

	// 获取要扫描的URL列表
	var urls []string
	if urlFile != "" {
		urls, err = readURLsFromFile(urlFile)
		if err != nil {
			fmt.Printf("读取URL文件错误: %v\n", err)
			os.Exit(1)
		}
		if len(urls) == 0 {
			fmt.Println("URL文件为空")
			os.Exit(1)
		}
	} else {
		urls = []string{url}
	}

	// 解析状态码
	var filterStatusCodes []int
	if statusCodes != "" {
		for _, code := range strings.Split(statusCodes, ",") {
			if code = strings.TrimSpace(code); code != "" {
				if statusCode, err := parseInt(code); err == nil {
					filterStatusCodes = append(filterStatusCodes, statusCode)
				}
			}
		}
	}

	// 解析要忽略的状态码
	var ignoreStatusCodes []int
	if ignoreCodes != "" {
		for _, code := range strings.Split(ignoreCodes, ",") {
			if code = strings.TrimSpace(code); code != "" {
				if statusCode, err := parseInt(code); err == nil {
					ignoreStatusCodes = append(ignoreStatusCodes, statusCode)
				}
			}
		}
	}

	// 为每个URL创建一个进度条组
	totalURLs := len(urls)
	fmt.Printf("\n共有 %d 个目标URL需要扫描\n", totalURLs)
	if len(filterStatusCodes) > 0 {
		fmt.Printf("将只显示以下响应码: %v\n", filterStatusCodes)
	}
	if len(ignoreStatusCodes) > 0 {
		fmt.Printf("将忽略以下响应码: %v\n", ignoreStatusCodes)
	} else {
		fmt.Println("将显示所有响应码")
	}
	fmt.Printf("并发线程数: %d\n", threads)
	fmt.Println("----------------------------------------")

	// 存储所有结果
	var allResults []ScanResult

	// 创建工作线程池
	type Job struct {
		baseURL string
		dir     string
	}

	// 扫描每个URL
	for i, baseURL := range urls {
		// 确保URL以/结尾
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}

		fmt.Printf("\n[%d/%d] 开始扫描 %s\n", i+1, totalURLs, baseURL)

		// 创建进度条
		bar := progressbar.NewOptions(len(paths),
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowCount(),
			progressbar.OptionSetWidth(15),
			progressbar.OptionSetDescription(fmt.Sprintf("[cyan][%d/%d][reset] 正在扫描目录...", i+1, totalURLs)),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}))

		// 创建任务通道和结果通道
		results := make(chan ScanResult, len(paths))
		jobs := make(chan Job, len(paths))
		var wg sync.WaitGroup

		// 启动工作线程
		for t := 0; t < threads; t++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for job := range jobs {
					scanURL(job.baseURL, job.dir, results)
				}
			}()
		}

		// 添加扫描任务
		for _, dir := range paths {
			jobs <- Job{baseURL: baseURL, dir: dir}
		}

		// 关闭任务通道
		close(jobs)

		// 等待所有扫描完成
		go func() {
			wg.Wait()
			close(results)
		}()

		// 收集结果并更新进度条
		var urlResults []ScanResult
		for result := range results {
			// 检查是否需要忽略该状态码
			if contains(ignoreStatusCodes, result.StatusCode) {
				bar.Add(1)
				continue
			}

			// 检查是否符合过滤条件
			if len(filterStatusCodes) == 0 || contains(filterStatusCodes, result.StatusCode) {
				urlResults = append(urlResults, result)
			}
			bar.Add(1)
		}

		// 将当前URL的结果添加到总结果中
		allResults = append(allResults, urlResults...)

		// 生成文件名并保存结果
		fileName, err := generateFileName(baseURL)
		if err != nil {
			fmt.Printf("生成文件名失败: %v\n", err)
			continue
		}

		// 添加.txt后缀
		fileName += ".txt"

		// 保存当前URL的扫描结果
		if err := saveResults(urlResults, fileName); err != nil {
			fmt.Printf("保存结果失败: %v\n", err)
			continue
		}

		fmt.Printf("\n结果已保存到: result/%s\n", fileName)
	}

	// 输出所有结果
	fmt.Println("\n\n扫描结果:")
	fmt.Printf("%-50s %-15s %-15s\n", "URL", "状态码", "响应时间")
	fmt.Println(strings.Repeat("-", 80))

	// 输出结果
	for _, result := range allResults {
		// 根据状态码设置不同的颜色
		statusColor := "[red]"
		if result.StatusCode >= 200 && result.StatusCode < 300 {
			statusColor = "[green]"
		} else if result.StatusCode >= 300 && result.StatusCode < 400 {
			statusColor = "[yellow]"
		}

		fmt.Printf("%-50s %s%-15d[reset] %-15s\n",
			result.URL,
			statusColor,
			result.StatusCode,
			result.Time.Round(time.Millisecond))
	}

	fmt.Printf("\n扫描完成！共发现 %d 个匹配的结果\n", len(allResults))
}

func scanURL(baseURL, dir string, results chan<- ScanResult) {
	url := baseURL + strings.TrimPrefix(dir, "/")
	startTime := time.Now()

	// 发送HTTP请求
	resp, err := http.Get(url)
	if err != nil {
		results <- ScanResult{
			URL:        url,
			StatusCode: 0,
			Time:       time.Since(startTime),
		}
		return
	}
	defer resp.Body.Close()

	results <- ScanResult{
		URL:        url,
		StatusCode: resp.StatusCode,
		Time:       time.Since(startTime),
	}
}

// 辅助函数：检查切片中是否包含某个值
func contains(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// 辅助函数：安全地解析整数
func parseInt(s string) (int, error) {
	return fmt.Sscanf(s, "%d")
}

// 从文件读取URL列表
func readURLsFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		url := strings.TrimSpace(scanner.Text())
		if url != "" && !strings.HasPrefix(url, "#") {
			urls = append(urls, url)
		}
	}

	return urls, scanner.Err()
}
