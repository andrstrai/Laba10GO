package main

// report.go — результаты поиска (пункт 6): результат одной пачки,
// итоговый отчёт и сравнение последовательного и параллельного способов.

import (
	"context"
	"fmt"
	"time"
)

// ChunkResult — итог одной пачки чисел. Воркер отправляет его в канал results.
type ChunkResult struct {
	WorkerID  int
	Primes    int     // сколько простых нашлось в пачке
	Processed int     // сколько чисел проверено
	Errs      []error // ошибки по некорректным числам
}

// Report — итоговый отчёт одного способа поиска.
type Report struct {
	Mode      string        // "последовательный" или "параллельный"
	RunID     int           // номер запуска (берётся из контекста)
	Total     int           // сколько чисел было в слайсе
	Processed int           // сколько чисел успели проверить
	Primes    int           // сколько простых нашли
	Errs      []error       // все ошибки
	Duration  time.Duration // время работы
	Workers   int           // сколько было воркеров (0 у последовательного)
	PerWorker map[int]int   // воркер -> сколько чисел обработал (только у параллельного)
	Cancelled error         // nil, если поиск дошёл до конца; иначе причина отмены
}

// newReport создаёт пустой отчёт; номер запуска берёт из контекста.
func newReport(ctx context.Context, mode string, total int) Report {
	return Report{Mode: mode, RunID: runIDFrom(ctx), Total: total}
}

// add прибавляет к отчёту результат одной пачки.
// Указатель *Report, потому что метод меняет сам отчёт (ЛР6, методы с указателем).
func (r *Report) add(c ChunkResult) {
	r.Primes += c.Primes
	r.Processed += c.Processed
	r.Errs = append(r.Errs, c.Errs...)
}

// maxShownErrors — сколько ошибок печатать подробно, остальные только считаем.
const maxShownErrors = 5

// Print выводит отчёт на экран.
func (r Report) Print() {
	fmt.Printf("\n[запуск %d] %s поиск\n", r.RunID, r.Mode)
	fmt.Printf("  простых чисел:     %d\n", r.Primes)
	fmt.Printf("  обработано чисел:  %d из %d\n", r.Processed, r.Total)
	fmt.Printf("  ошибок:            %d\n", len(r.Errs))
	fmt.Printf("  время:             %v\n", r.Duration.Round(time.Millisecond))

	if r.Cancelled != nil {
		fmt.Printf("  поиск %s: %v\n", reasonText(r.Cancelled), r.Cancelled)
	}

	for i, err := range r.Errs {
		if i == maxShownErrors {
			fmt.Printf("    ... и ещё %d\n", len(r.Errs)-maxShownErrors)
			break
		}
		fmt.Println("    ошибка:", err)
	}

	if r.Workers > 0 {
		fmt.Print("  чисел на воркер:  ")
		for id := 1; id <= r.Workers; id++ {
			fmt.Printf(" #%d=%d", id, r.PerWorker[id])
		}
		fmt.Println()
	}
}

// PrintComparison сравнивает два способа по времени (time.Since уже посчитан в отчётах).
func PrintComparison(seq, par Report) {
	fmt.Println("\nСравнение:")
	if seq.Cancelled != nil || par.Cancelled != nil {
		// Сравнивать время незаконченных поисков нечестно, поэтому только сообщаем.
		if seq.Cancelled != nil && par.Cancelled == nil {
			fmt.Printf("  последовательный поиск %s, а параллельный успел за %v\n",
				reasonText(seq.Cancelled), par.Duration.Round(time.Millisecond))
		} else {
			fmt.Println("  один из поисков не завершился, сравнить время нельзя")
		}
		return
	}
	if seq.Primes != par.Primes {
		fmt.Printf("  ВНИМАНИЕ: результаты не совпадают (%d и %d)\n", seq.Primes, par.Primes)
	}
	speedup := float64(seq.Duration) / float64(par.Duration)
	fmt.Printf("  последовательно: %v, параллельно: %v\n",
		seq.Duration.Round(time.Millisecond), par.Duration.Round(time.Millisecond))
	fmt.Printf("  параллельный поиск быстрее в %.2f раза\n", speedup)
}
