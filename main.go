package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

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

// Функция для последовательного поиска простых чисел в слайсе.
// ctx нужен, чтобы поиск можно было прервать (Ctrl+C или таймаут)
// и чтобы checkNumber знала номер запуска.
func SequentialSearch(ctx context.Context, slice []int) Report {
	report := newReport(ctx, "последовательный", len(slice))
	start := time.Now()
	for i := 0; i < len(slice); i++ {
		// Перед каждым числом проверяем, не отменён ли контекст.
		// ctx.Err() == nil, пока всё в порядке.
		if ctx.Err() != nil {
			report.Cancelled = ctx.Err()
			break
		}
		/* Заменил на долгую проверку для увеличения времени работы
		if isPrime(slice[i]) {
			simpleCount++
		}
		*/
		prime, err := checkNumber(ctx, slice[i])
		report.Processed++
		if err != nil {
			report.Errs = append(report.Errs, err)
			continue
		}
		if prime {
			report.Primes++
		}
	}
	report.Duration = time.Since(start)
	return report
}

// Реализация паттерна Worker Pool - функция обрабатывает пачки чисел и проверяет их простоту.
// Итог каждой пачки отправляется в канал results, а общая статистика
// "сколько чисел обработал каждый воркер" пишется в map под мьютексом.
func workerForParallel(ctx context.Context, id int, nums <-chan []int, results chan<- ChunkResult,
	stats map[int]int, wg *sync.WaitGroup, mu *sync.Mutex) {
	defer wg.Done() // сработает при любом выходе из функции, в том числе по отмене

	// Кладём номер воркера в контекст: checkNumber достанет его для текста ошибки.
	ctx = context.WithValue(ctx, keyWorkerID, id)

	for {
		select {
		case <-ctx.Done(): // контекст отменён — выходим, новые пачки не берём
			return
		case chunk, open := <-nums:
			if !open { // канал закрыт и пуст — пачек больше не будет
				return
			}

			// Локальный результат пачки
			res := ChunkResult{WorkerID: id}

			// Считаем пакет целиком
			for _, num := range chunk {
				if ctx.Err() != nil { // отмена посреди пачки — не досчитываем её
					break
				}
				/* Заменил на долгую проверку для увеличения времени работы
				if isPrime(num) {
					localSimple++
				}
				*/
				prime, err := checkNumber(ctx, num)
				res.Processed++
				if err != nil {
					res.Errs = append(res.Errs, err)
					continue
				}
				if prime {
					res.Primes++
				}
			}

			// Map не потокобезопасна: без мьютекса будет "concurrent map writes".
			// Блокируем мьютекс ОДИН раз на весь пакет (чанк), а не на каждое число!
			mu.Lock()
			stats[id] += res.Processed
			mu.Unlock()

			// Отправляем итог пачки. Здесь не нужен select с ctx.Done():
			// ParallelSearch читает results до самого закрытия канала,
			// поэтому отправка всегда дождётся читателя, даже после отмены.
			// Зато недосчитанная пачка тоже попадёт в отчёт.
			results <- res
		}
	}
}

// Функция для параллельного поиска простых чисел в слайсе
func ParallelSearch(ctx context.Context, slice []int) Report {
	report := newReport(ctx, "параллельный", len(slice))

	var waitParralel sync.WaitGroup
	var mutexForParallel sync.Mutex
	numsForCheck := make(chan []int)  // пачки чисел для воркеров
	results := make(chan ChunkResult) // итоги пачек от воркеров
	cntWorker := runtime.NumCPU()
	report.Workers = cntWorker
	report.PerWorker = make(map[int]int) // общая статистика, её защищает мьютекс
	chunkSize := len(slice) / (cntWorker * 4)
	if chunkSize < 1 {
		chunkSize = 1
	} else if chunkSize > 1000 {
		chunkSize = 1000
	}

	start := time.Now()
	for i := 1; i <= cntWorker; i++ {
		waitParralel.Add(1)
		go workerForParallel(ctx, i, numsForCheck, results, report.PerWorker, &waitParralel, &mutexForParallel)
	}

	// Раздатчик: режет слайс на пачки и отправляет воркерам.
	// Канал закрывает тот, кто в него пишет, — то есть раздатчик, ровно один раз.
	go func() {
		defer close(numsForCheck) // закроется и при обычном конце, и при отмене
		for i := 0; i < len(slice); i += chunkSize {
			end := i + chunkSize
			if end > len(slice) {
				end = len(slice)
			}

			select {
			case numsForCheck <- slice[i:end]: // Отправляем кусок слайса
			case <-ctx.Done(): // отмена — перестаём раздавать
				return
			}
		}
	}()

	// Ждём всех воркеров в ОТДЕЛЬНОЙ горутине и только потом закрываем results.
	// Если ждать прямо здесь, будет deadlock: воркер стоит на results <- res
	// и ждёт читателя, а читатель (мы) стоит на Wait() и ждёт воркера.
	go func() {
		waitParralel.Wait()
		close(results)
	}()

	// Сбор результатов из канала: цикл закончится, когда results закроют.
	for res := range results {
		report.add(res)
	}

	report.Duration = time.Since(start)
	// Если обработали не всё, значит, поиск отменили; запоминаем причину.
	if report.Processed < report.Total {
		report.Cancelled = ctx.Err()
	}
	return report
}

// runSearch запускает оба поиска и печатает отчёты.
// timeout — ограничение времени на КАЖДЫЙ способ (0 — без ограничения).
func runSearch(slice []int, runID int, timeout time.Duration) {
	// Контекст отменится, если нажать Ctrl+C. Пока он действует,
	// Ctrl+C не убивает программу, а только отменяет поиск.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	// stop возвращает обычное поведение Ctrl+C (выход из программы в меню).
	defer stop()

	// Передаём номер запуска всем функциям поиска через контекст.
	ctx = context.WithValue(ctx, keyRunID, runID)

	if timeout > 0 {
		fmt.Printf("Ограничение времени: %v на каждый способ.\n", timeout)
	}
	fmt.Println("Ctrl+C — прервать поиск и вернуться в меню.")

	fmt.Println("\nПоиск последовательным способом в процессе...")
	seqCtx, cancelSeq := withTimeout(ctx, timeout)
	resultSeq := SequentialSearch(seqCtx, slice)
	cancelSeq() // освобождаем таймер контекста сразу, не дожидаясь выхода из функции
	resultSeq.Print()

	// Ctrl+C отменяет общий ctx: тогда параллельный поиск не запускаем.
	// (Таймаут отменяет только seqCtx, параллельный всё равно пробуем.)
	if ctx.Err() != nil {
		fmt.Println("\nПараллельный поиск не запускаю:", reasonText(ctx.Err()))
		return
	}

	fmt.Println("\nПоиск параллельным способом в процессе...")
	parCtx, cancelPar := withTimeout(ctx, timeout)
	defer cancelPar()
	resultPar := ParallelSearch(parCtx, slice)
	resultPar.Print()

	PrintComparison(resultSeq, resultPar)
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

// CorruptSlice записывает в k случайных мест некорректные числа (0 и отрицательные),
// чтобы проверить обработку ошибок. Возвращает позиции, которые испортили.
func CorruptSlice(slice []int, k int) []int {
	positions := []int{}
	for i := 0; i < k; i++ {
		pos := rand.IntN(len(slice))
		slice[pos] = -rand.IntN(100) // от -99 до 0
		positions = append(positions, pos)
	}
	return positions
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
	runID := 0 // номер запуска поиска, передаётся через context
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
		fmt.Println("4 - Поиск с ограничением по времени")
		fmt.Println("5 - Испортить несколько чисел (проверка ошибок)")
		fmt.Println("0 - Завершить работу")

		a, err := readInt(reader, "Выберите пункт из меню управления:")
		if err != nil {
			fmt.Println(err)
			continue
		}
		switch a {
		case 0:
			fmt.Println("Работа завершена.")
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
			runID++
			runSearch(Slice, runID, 0)
		case 3:
			PrintSlice(Slice[:100])
		case 4:
			sec, err := readInt(reader, "Сколько секунд даём каждому способу? (от 1 до 60)")
			if err != nil || sec < 1 || sec > 60 {
				fmt.Println("Некорректный ввод. Пожалуйста, введите число от 1 до 60.")
				continue
			}
			runID++
			runSearch(Slice, runID, time.Duration(sec)*time.Second)
		case 5:
			positions := CorruptSlice(Slice, 5)
			fmt.Println("Испорчены числа на позициях:", positions)
			fmt.Println("Запустите поиск (пункт 2): ошибки попадут в отчёт. Пункт 1 создаст чистый слайс.")
		default:
			fmt.Println("Нет такого пункта меню!")
		}
	}
}
