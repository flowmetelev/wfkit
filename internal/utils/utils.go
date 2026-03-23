package utils

import (
	"fmt"

	"github.com/fatih/color"
)

// CPrint выводит цветное сообщение в консоль
func CPrint(msg, colorName string) {
	switch colorName {
	case "blue":
		color.Blue(msg) // Используем метод color.Blue напрямую
	case "red":
		color.Red(msg)
	case "green":
		color.Green(msg)
	case "yellow":
		color.Yellow(msg)
	default:
		fmt.Println(msg) // Без цвета, если цвет не указан или неизвестен
	}
}
