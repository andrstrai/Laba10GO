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
	"sync"
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

// Предлагаю не использовать, так как очень быстро происходит поиск и результат выходит менее наглядный
/*
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
*/

func isPrimeStupid(n int) bool {
	if n <= 1 {
		return false
	}
	if n <= 3 {
		return true
	}
	for i := 2; i < n; i++ {
		if n%i == 0 {
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
		/* Заменил на долгую проверку для увеличения времени работы
		if isPrime(slice[i]) {
			simpleCount++
		}
		*/
		if isPrimeStupid(slice[i]) {
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

// Реализация паттерна Worker Pool - функция обрабатывает пачки чисел и проверяет их простоту
func workerForParallel(nums <-chan []int, res *SearchResult, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done()
	for {
		select {
		case chunk, open := <-nums:
			if !open {
				return
			}

			// Локальные счетчики воркера
			localSimple := 0
			localProcessed := 0

			// Считаем пакет целиком
			for _, num := range chunk {
				/* Заменил на долгую проверку для увеличения времени работы
				if isPrime(num) {
					localSimple++
				}
				*/
				if isPrimeStupid(num) {
					localSimple++
				}
				localProcessed++
			}

			// Блокируем мьютекс ОДИН раз на весь пакет (чанк), а не на каждое число!
			mu.Lock()
			res.SimpleCount += localSimple
			res.ProcessedCount += localProcessed
			mu.Unlock()
		}
	}

}

// Функция для параллельного поиска простых чисел в слайсе
func ParallelSearch(slice []int) SearchResult {
	result := SearchResult{
		SimpleCount:    0,
		TimeSpent:      0,
		ProcessedCount: 0,
	}

	var waitParralel sync.WaitGroup
	var mutexForParallel sync.Mutex
	numsForCheck := make(chan []int)
	cntWorker := runtime.NumCPU()
	chunkSize := len(slice) / (cntWorker * 4)
	if chunkSize < 1 {
		chunkSize = 1
	} else if chunkSize > 1000 {
		chunkSize = 1000
	}

	start := time.Now()
	for i := 0; i < cntWorker; i++ {
		waitParralel.Add(1)
		go workerForParallel(numsForCheck, &result, &waitParralel, &mutexForParallel)
	}

	go func() {
		for i := 0; i < len(slice); i += chunkSize {
			end := i + chunkSize
			if end > len(slice) {
				end = len(slice)
			}

			select {
			case numsForCheck <- slice[i:end]: // Отправляем кусок слайса
			}
		}
		close(numsForCheck)
	}()

	waitParralel.Wait()
	duration := time.Since(start)
	result.TimeSpent = duration.Nanoseconds()
	return result
}

// Функция заполнения среза n случайными данными
func RandomFill(n int) []int {
	slice := []int{}
	for i := 0; i < n; i++ {
		c := rand.IntN(1000000)
		if c == 0 {
			slice = append(slice, 1)
		} else {
			slice = append(slice, c)
		}
	}
	return slice
}

func PrintSlice(slice []int) {
	cols := int(math.Ceil(math.Sqrt(float64(len(slice)))))
	fmt.Println(" Первые 100 чисел слайса: ")
	for i := 0; i < len(slice); i++ {
		if i%cols == 0 && i != 0 {
			fmt.Println("")
		}
		fmt.Printf("%6d ", slice[i])
	}
}

// основная функция
func main() {
	Slice := RandomFill(1000)
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Println("\nДля продолжения нажмите Enter...")
		if _, err := reader.ReadString('\n'); err != nil {
			return // ввод закончился (Ctrl+D) — выходим, а не крутимся вечно
		}
		clear_screen()

		fmt.Println("Меню команд:")
		fmt.Println("1 - Создать слайс чисел")
		fmt.Println("2 - Начать поиск простых чисел в слайсе")
		fmt.Println("3 - Вывести первые числа слайса")
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
			fmt.Println("Сколько элементов добавить в слайс?(от 1000 до 100000)")
			n, err := readInt(reader, "Введите количество элементов: ")
			if err != nil || n < 1000 || n > 100000 {
				fmt.Println("Некорректный ввод. Пожалуйста, введите число от 1000 до 100000.")
				continue
			}
			Slice = RandomFill(n)
			fmt.Println("Слайс создан!")
			// Вычисляем количество столбцов для квадратной матрицы

		case 2:
			fmt.Println("Поиск простых чисел в слайсе:")

			fmt.Println("Поиск последовательным способом в процессе...")
			resultSeq := SequentialSearch(Slice)

			fmt.Println("Поиск параллельным способом в процессе...")
			resultPar := ParallelSearch(Slice)

			fmt.Printf("\nРезультат последовательного поиска: Кол-во простых = %d, Время поиска = %d нс, Кол-во обработанных чисел = %d\n", resultSeq.SimpleCount, resultSeq.TimeSpent, resultSeq.ProcessedCount)
			fmt.Printf("    Результат параллельного поиска: Кол-во простых = %d, Время поиска = %d нс, Кол-во обработанных чисел = %d\n", resultPar.SimpleCount, resultPar.TimeSpent, resultPar.ProcessedCount)
		case 3:
			PrintSlice(Slice[:100])
		default:
			fmt.Println("Нет такого пункта меню!")
		}
	}
}
