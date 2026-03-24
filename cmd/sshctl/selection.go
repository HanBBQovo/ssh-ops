package main

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/hanhan/ssh-ops/internal/sshops"
)

func chooseHost(reader *bufio.Reader, out io.Writer, hosts []sshops.HostConfig, prompt string) (*sshops.HostConfig, error) {
	if len(hosts) == 0 {
		return nil, sshops.NewUserError("unknown_host", "还没有保存任何服务器", nil)
	}
	if len(hosts) == 1 {
		host := hosts[0]
		return &host, nil
	}

	fmt.Fprintln(out, "已保存的服务器：")
	for i, host := range hosts {
		target := fmt.Sprintf("%s@%s:%d", host.User, host.Address, host.Port)
		if strings.TrimSpace(host.Name) != "" {
			fmt.Fprintf(out, "  %d) %s  %s  (%s)\n", i+1, host.ID, target, host.Name)
		} else {
			fmt.Fprintf(out, "  %d) %s  %s\n", i+1, host.ID, target)
		}
	}

	for {
		choice, err := promptRequired(reader, out, prompt, "")
		if err != nil {
			return nil, err
		}
		if index, convErr := strconv.Atoi(strings.TrimSpace(choice)); convErr == nil {
			if index >= 1 && index <= len(hosts) {
				host := hosts[index-1]
				return &host, nil
			}
		}
		for i := range hosts {
			if matchesHostChoice(hosts[i], choice) {
				host := hosts[i]
				return &host, nil
			}
		}
		fmt.Fprintln(out, "没有找到这个服务器，请输入序号、别名或显示名称。")
	}
}

func promptEditable(reader *bufio.Reader, out io.Writer, label, currentValue string, allowClear bool) (string, bool, error) {
	currentText := fmt.Sprintf("[%s]", currentValue)
	if strings.TrimSpace(currentValue) == "" {
		currentText = "[当前为空]"
	}
	suffix := "回车保持不变"
	if allowClear {
		suffix += "，输入 - 清空"
	}
	fmt.Fprintf(out, "%s %s（%s）: ", label, currentText, suffix)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", false, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return currentValue, false, nil
	}
	if allowClear && line == "-" {
		return "", true, nil
	}
	return line, true, nil
}

func matchesHostChoice(host sshops.HostConfig, choice string) bool {
	value := strings.TrimSpace(choice)
	if value == "" {
		return false
	}
	if host.ID == value {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(host.Name), value)
}
