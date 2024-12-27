package jobs_test

import (
	"fmt"
	"hsf/src/jobs"
	"time"
)

func ExampleJob() {
	job := jobs.New("Example Job")
	go func() {
		for {
			fmt.Println("Working...")
			time.Sleep(time.Second * 1)

			select {
			case <-job.Canceled():
				fmt.Println("Canceled! Shutting down.")
				job.Finish()
				return
			default:
			}
		}
	}()

	time.Sleep(time.Millisecond * 2500)
	job.Cancel()
	<-job.Finished()

	fmt.Println("Background job complete.")

	// Output:
	// Working...
	// Working...
	// Working...
	// Canceled! Shutting down.
	// Background job complete.
}

func ExampleJobs() {
	backgroundJobs := jobs.Jobs{
		Countdown(3, time.Millisecond*0),
		Countdown(1000, time.Millisecond*100),
	}

	time.Sleep(time.Millisecond * 500)
	fmt.Println("Shutting down background jobs...")
	unfinished := backgroundJobs.CancelAndWait(time.Second * 5)
	fmt.Println("The following jobs did not finish:")
	for _, j := range unfinished {
		fmt.Printf("- %s\n", j)
	}

	// Output:
	// 3!
	// 1000!
	// Shutting down background jobs...
	// The countdown cannot be stopped! You cannot stop me!
	// The countdown cannot be stopped! You cannot stop me!
	// 2!
	// 999!
	// 1!
	// 998!
	// 0!
	// 997!
	// Countdown from 3 complete.
	// 996!
	// 995!
	// The following jobs did not finish:
	// - Countdown from 1000
}

func Countdown(from int, after time.Duration) *jobs.Job {
	job := jobs.New(fmt.Sprintf("Countdown from %d", from))
	time.Sleep(after) // hack to make tests execute more predictably

	go func() {
		for i := from; i >= 0; i-- {
			fmt.Printf("%d!\n", i)
			time.Sleep(time.Second * 1)
		}
		fmt.Printf("Countdown from %d complete.\n", from)
		job.Finish()
	}()
	go func() {
		<-job.Canceled()
		fmt.Println("The countdown cannot be stopped! You cannot stop me!")
	}()

	return job
}
