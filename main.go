package main

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type SearchResult struct {
	SimpleCount    int
	TimeSpent      int64
	ProcessedCount int
}

// функция для очистки экрана (работает и на Windows, и на macOS/Linux)
func clear_screen() {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "cls")
	} else {
		cmd = exec.Command("clear")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// readInt читает целое число (пункт меню, ID)
func readInt(reader *bufio.Reader, prompt string) (int, error) {
	fmt.Println(prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("не удалось прочитать ввод: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(input))
	if err != nil {
		return 0, errors.New("нужно ввести целое число")
	}
	return n, nil
}

func isPrime(n int) bool {
	if n <= 1 {
		return false
	}
	if n <= 3 {
		return true
	}
	if n%2 == 0 || n%3 == 0 {
		return false
	}
	for i := 5; i*i <= n; i += 6 {
		if n%i == 0 || n%(i+2) == 0 {
			return false
		}
	}
	return true
}

// Функция для последовательного поиска простых чисел в слайсе
func SequentialSearch(slice []int) SearchResult {
	start := time.Now()
	simpleCount := 0
	for i := 0; i < len(slice); i++ {
		if isPrime(slice[i]) {
			simpleCount++
		}
	}
	duration := time.Since(start)
	return SearchResult{
		SimpleCount:    simpleCount,
		TimeSpent:      duration.Nanoseconds(),
		ProcessedCount: len(slice),
	}
}

// Функция для параллельного поиска простых чисел в слайсе
func ParallelSearch(slice []int, numWorkers int) SearchResult {
	// нужна функция для параллельного поиска
	totalPrimes := 0
	processedCount := 0
	return SearchResult{
		SimpleCount:    totalPrimes,
		TimeSpent:      1,
		ProcessedCount: processedCount,
	}
}

// Функция заполнения среза n случайными данными
func RandomFill(n int) []int {
	slice := []int{}
	for i := 0; i < n; i++ {
		c := rand.IntN(10000)
		if c == 0 {
			slice = append(slice, 1)
		} else {
			slice = append(slice, c)
		}
	}
	return slice
}

// основная функция
func main() {
	Slice := RandomFill(10)
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\nДля продолжения нажмите Enter...")
		if _, err := reader.ReadString('\n'); err != nil {
			return // ввод закончился (Ctrl+D) — выходим, а не крутимся вечно
		}
		clear_screen()

		fmt.Println("Меню команд:")
		fmt.Println("1 - Создать и вывести слайс чисел")
		fmt.Println("2 - Начать поиск простых чисел в слайсе")
		fmt.Println("0 - Завершить работу")

		a, err := readInt(reader, "Выберите пункт из меню управления:")
		if err != nil {
			fmt.Println(err)
			continue
		}
		switch a {
		case 0:
			return
		case 1:
			fmt.Println("Сколько элементов добавить в слайс?(от 2 до 100000)")
			n, err := readInt(reader, "Введите количество элементов: ")
			if err != nil || n < 2 || n > 100000 {
				fmt.Println("Некорректный ввод. Пожалуйста, введите число от 2 до 100000.")
				continue
			}
			Slice = RandomFill(n)
			// Вычисляем количество столбцов для квадратной матрицы
			cols := int(math.Ceil(math.Sqrt(float64(n))))
			fmt.Println("Слайс чисел: ")
			for i := 0; i < len(Slice); i++ {
				if i%cols == 0 && i != 0 {
					fmt.Println("")
				}
				fmt.Printf("%6d ", Slice[i])
			}
		case 2:
			fmt.Println("Поиск простых чисел в слайсе:")

			result := SequentialSearch(Slice)
			fmt.Printf("Результат последовательного поиска: TotalSimple=%d, Time=%d нс, Processed=%d\n", result.SimpleCount, result.TimeSpent, result.ProcessedCount)

			result1 := ParallelSearch(Slice, runtime.NumCPU())
			fmt.Printf("Результат параллельного поиска: TotalSimple=%d, Time=%d нс, Processed=%d\n", result1.SimpleCount, result1.TimeSpent, result1.ProcessedCount)
		default:
			fmt.Println("Нет такого пункта меню!")
		}
	}
}
