package tools

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SpeakTool provides text-to-speech capabilities using eSpeak
type SpeakTool struct{}

// Name returns the tool name
func (t *SpeakTool) Name() string {
	return "speak"
}

// Description returns what the tool does
func (t *SpeakTool) Description() string {
	return "Convert text to speech and play it using eSpeak. Example: 'Hello, how are you?'"
}

// Execute converts text to speech and plays it
func (t *SpeakTool) Execute(ctx context.Context, text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("empty text")
	}

	// 打印调试信息
	println("Speak tool received text:", text)

	// 检查eSpeak是否可用
	println("Checking eSpeak availability...")
	checkCmd := exec.Command("powershell.exe", "-Command", "espeak --version")
	var checkOut, checkErr bytes.Buffer
	checkCmd.Stdout = &checkOut
	checkCmd.Stderr = &checkErr
	checkErrResult := checkCmd.Run()
	println("eSpeak version check result:", checkErrResult)
	println("Version stdout:", checkOut.String())
	println("Version stderr:", checkErr.String())

	// 尝试使用PowerShell执行eSpeak命令
	println("Trying eSpeak via PowerShell...")

	// 尝试的语音参数
	voiceParams := []string{
		"zh",
		"",
	}

	for _, voiceParam := range voiceParams {
		// 构建PowerShell命令
		var psCommand string
		if voiceParam != "" {
			println("Trying with voice:", voiceParam)
			// 注意：在PowerShell中需要正确转义引号
			psCommand = fmt.Sprintf("espeak -v %s '%s'", voiceParam, text)
		} else {
			println("Trying without voice parameter...")
			psCommand = fmt.Sprintf("espeak '%s'", text)
		}

		println("PowerShell command:", psCommand)

		// 设置超时
		cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		// 创建PowerShell命令
		psCmd := exec.CommandContext(cmdCtx, "powershell.exe", "-Command", psCommand)
		var psOut, psErr bytes.Buffer
		psCmd.Stdout = &psOut
		psCmd.Stderr = &psErr

		// 执行命令
		err := psCmd.Run()
		if err != nil {
			println("PowerShell command failed:", err.Error())
			println("Command stdout:", psOut.String())
			println("Command stderr:", psErr.String())
		} else {
			println("PowerShell command executed successfully!")
			println("Voice:", voiceParam)
			println("Command stdout:", psOut.String())
			println("Command stderr:", psErr.String())
			return fmt.Sprintf("Text spoken successfully: %s", text), nil
		}
	}

	// 尝试使用更简单的PowerShell命令
	println("Trying simple PowerShell command...")
	simplePsCmd := exec.Command("powershell.exe", "-Command", "espeak 'Hello'")
	var simplePsOut, simplePsErr bytes.Buffer
	simplePsCmd.Stdout = &simplePsOut
	simplePsCmd.Stderr = &simplePsErr

	err := simplePsCmd.Run()
	if err != nil {
		println("Simple PowerShell command failed:", err.Error())
		println("Command stdout:", simplePsOut.String())
		println("Command stderr:", simplePsErr.String())
		// 如果所有尝试都失败，返回错误
		return "", fmt.Errorf("all eSpeak attempts failed")
	} else {
		println("Simple PowerShell command executed successfully!")
		println("Command stdout:", simplePsOut.String())
		println("Command stderr:", simplePsErr.String())
		return fmt.Sprintf("Text spoken successfully: Hello"), nil
	}
}
