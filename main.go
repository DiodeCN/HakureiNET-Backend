package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var idToAddrMap = make(map[string]*net.UDPAddr) // 储存id和ip的map
var idToPassMap = make(map[string]string)       // 存储id和密码的map
var environmentData = make(map[string]string)   //储存环境的map
var udpConn *net.UDPConn
var mapMutex = &sync.Mutex{} // 防止map被抢
var Shidu string
var Wendu string
var Guanggan string
var zaoyin string

type ScheduledTask struct {
	Command  string
	Time     string
	Days     []string
	Id       string
	Weekdays map[string]bool
}

var scheduledTasks []ScheduledTask

// 其实这么多好像可以直接简写的（小声

func main() {
	// http.Handle("/", http.FileServer(http.Dir("./assests")))
	//http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("./assests")))) // 发布前端
	http.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir("./assests/"))))
	// 这里挣扎了很久，最终决定拿nginx做静态资源服务器。其实go-bindata-assetfs的解决方案也不错

	udpAddr := &net.UDPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: 30000,
	}
	var err error
	udpConn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer udpConn.Close()

	//开启udp和http的goroutines
	go handleUDP(udpConn)

	http.HandleFunc("/ws", wsEndpoint)

	fmt.Println("Starting ws&http server on localhost:6280")
	fmt.Println("Starting udpserver on localhost:30000")
	log.Fatal(http.ListenAndServe(":6280", nil))

	// Wait forever
	select {}
}

var Keys string

func handleUDP(conn *net.UDPConn) {
	// 在这里添加一个新的 goroutine 来处理计划任务
	go func() {
		for {
			for _, task := range scheduledTasks {
				if shouldExecuteTask(task) {
					mapMutex.Lock()
					addr, ok := idToAddrMap[task.Id]
					mapMutex.Unlock()

					if ok {
						var taskErr error // 添加一个新的 error 变量
						_, taskErr = udpConn.WriteToUDP([]byte(task.Command), addr)
						if taskErr != nil {
							log.Println("Error sending UDP message:", taskErr)
						}
					} else {
						log.Println("Id not found in the map")
					}
				}
			}
			// time.Sleep(1 * time.Minute)
		}
	}()

	for {
		var data [1024]byte
		n, addr, err := conn.ReadFromUDP(data[:]) // 接收数据
		if err != nil {
			fmt.Println("读udp消息失败：", err)
			continue
		}
		fmt.Printf("UDP data: %v, addr: %v, count: %v\n", string(data[:n]), addr, n)

		if string(data[:n]) == "尝试连接服务器" {
			_, err = conn.WriteToUDP([]byte("服务器连接成功"), addr) // 发送数据
			if err != nil {
				fmt.Println("udp发送错误：", err)
				continue
			}
		} else if strings.Contains(string(data[:n]), "&init;") {
			id := strings.Replace(string(data[:n]), "&init;", "", -1)
			ids, err := strconv.ParseInt(id, 10, 64)
			rand.Seed(int64(ids))
			if err != nil {
				fmt.Println("转换错误：", err)
				continue
			}
			pass := fmt.Sprintf("%06d", rand.Intn(1000000))
			password := "Password：" + pass
			_, err = conn.WriteToUDP([]byte(password), addr)
			if err != nil {
				fmt.Println("udp发送错误：", err)
				continue
			}
			var Key = id + pass
			Keys = Key
			log.Println(Key)

			// Protect the access to the idToAddrMap with a Mutex
			mapMutex.Lock()
			idToAddrMap[id] = addr // Store the id and addr in the map
			mapMutex.Unlock()

			// Protect the access to the idToPassMap with a Mutex
			mapMutex.Lock()
			idToPassMap[id] = pass // Store the id and pass in the map
			mapMutex.Unlock()

		}

		if strings.Contains(string(data[:n]), "湿度：") {
			environmentData["shidu"] = strings.Replace(string(data[:n]), "湿度：", "", -1)
		}
		if strings.Contains(string(data[:n]), "温度：") {
			environmentData["wendu"] = strings.Replace(string(data[:n]), "温度：", "", -1)
		}
		if strings.Contains(string(data[:n]), "光感：") {
			environmentData["guanggan"] = strings.Replace(string(data[:n]), "光感：", "", -1)
		}
		if strings.Contains(string(data[:n]), "噪音：") {
			environmentData["zaoyin"] = strings.Replace(string(data[:n]), "噪音：", "", -1)
		}

	}

}

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	// 将 HTTP 连接升级为 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}
	// 保证在退出函数时关闭 WebSocket 连接
	defer conn.Close()

	for {
		// 读取消息
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}
		// 打印消息
		log.Println("TCP data:", string(p))

		// 非常重要的，读前五位数的map，这个[:5]省非常多的事
		mapMutex.Lock()
		storedPass, ok := idToPassMap[string(p)[:5]]
		mapMutex.Unlock()

		if ok && storedPass == string(p)[5:] {
			log.Println("验证成功")
			log.Println("Key", string(p))
			if err := conn.WriteMessage(messageType, []byte("验证成功"+string(p))); err != nil {
				log.Println(err)
				return
			}
		} else {
			if Keys == "" {
				log.Println("请增加掌控板并初始化")
				if err := conn.WriteMessage(messageType, []byte("请增加掌控板并初始化")); err != nil {
					log.Println(err)
					return
				}
			}

		}

		parts := strings.Split(string(p), "|")

		// handleSpecialMessage(conn, messageType, p)
		if len(parts) == 4 {
			command := parts[0]
			time := parts[1]
			days := strings.Split(parts[2], ",")
			id := parts[3]

			weekdays := map[string]bool{}
			for _, day := range days {
				weekdays[day] = true
			}

			task := ScheduledTask{
				Command:  command,
				Time:     time,
				Days:     days,
				Id:       id,
				Weekdays: weekdays,
			}

			scheduledTasks = append(scheduledTasks, task)
		}

		if len(parts) == 3 {

			//我很愚蠢，git全局设置弄错了，现在ok了
			// 提取指令、id和时长
			command := parts[0]
			durationStr := parts[1]
			id := parts[2]
			log.Println("已新建让", id, "执行", command, "的计时器，", "剩余时长", durationStr, "分钟。")

			// 将时长字符串转换为整数
			durationMinutes, err := strconv.Atoi(durationStr)
			if err != nil {
				log.Println("转化错误", err)
				continue
			}

			// 创建计时器
			timer := time.NewTimer(time.Duration(durationMinutes) * time.Minute)

			go func(id string) {
				<-timer.C

				mapMutex.Lock()
				addr, ok := idToAddrMap[id]
				mapMutex.Unlock()

				if ok {
					message := command
					_, err = udpConn.WriteToUDP([]byte(message), addr)
					if err != nil {
						log.Println("Error sending UDP message:", err)
					}
				} else {
					log.Println("Id not found in the map")
				}
			}(id)

		}
		if len(parts) != 4 && len(parts) != 3 {
			if strings.Contains(string(p), "&open;") {
				log.Println("打开开关")

				// 提取id
				id := strings.Replace(string(p), "&open;", "", -1)

				mapMutex.Lock()
				addr, ok := idToAddrMap[id]
				mapMutex.Unlock()

				if ok {
					_, err = udpConn.WriteToUDP([]byte(p), addr)
					if err != nil {
						log.Println("Error sending UDP message:", err)
					}
				} else {
					log.Println("Id not found in the map")
				}
			}

			// 和上面的差不多
			if strings.Contains(string(p), "&shut;") {
				log.Println("关闭开关")

				id := strings.Replace(string(p), "&shut;", "", -1)

				mapMutex.Lock()
				addr, ok := idToAddrMap[id]
				mapMutex.Unlock()

				if ok {
					_, err = udpConn.WriteToUDP([]byte(p), addr)
					if err != nil {
						log.Println("Error sending UDP message:", err)
					}
				} else {
					log.Println("Id not found in the map")
				}
			}

			if strings.Contains(string(p), "&*") && strings.Contains(string(p), "/") {

				parts := strings.Split(string(p), "/")
				value := parts[0][2:] // 把&*删去
				id := parts[1]        // 取id

				log.Println("风扇电压：", value)
				mapMutex.Lock()
				addr, ok := idToAddrMap[id]
				mapMutex.Unlock()

				if ok {
					message := "8964" + value
					/* 没办法，那个掌控板我也不知道怎么搞得
					string转数字只能用那个int模块。那个int指令吧
					一有字符或者汉字这种就罢工。我也查不明白怎么回事
					直接拿数字做个判断加减得了（乐
					*/
					_, err = udpConn.WriteToUDP([]byte(message), addr)
					if err != nil {
						log.Println("Error sending UDP message:", err)
					}
				} else {
					log.Println("Id not found in the map")
				}
			}

			if strings.Contains(string(p), "&askforstate;") {

				// 从map中获取温度、湿度、光感和噪音数据
				Shidu = environmentData["shidu"]
				Wendu = environmentData["wendu"]
				Guanggan = environmentData["guanggan"]
				zaoyin = environmentData["zaoyin"]

				message := Shidu + "/" + Wendu + "/" + Guanggan + "/" + zaoyin

				if message == "///" {
					log.Println("还没有绑定过掌控板！")
				} else {
					log.Println("新状态上传成功", message)
				}

				if err := conn.WriteMessage(messageType, []byte(message)); err != nil {
					log.Println(err)
					return
				}

				if err != nil {
					log.Println("Error sending TCP message:", err)
				}
			}

		}

	}
}
func shouldExecuteTask(task ScheduledTask) bool {
	now := time.Now()
	weekday := now.Weekday().String()
	if !task.Weekdays[weekday] {
		return false
	}

	taskTime, err := time.Parse("15:04", task.Time)
	if err != nil {
		log.Println("Error parsing task time:", err)
		return false
	}

	return now.Hour() == taskTime.Hour() && now.Minute() == taskTime.Minute()
}
