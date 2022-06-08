package main

import (
    "log"
    "sync"
    "time"
)

type Pool struct {
    workerNum int
    road      chan *Car
    wg        sync.WaitGroup
}

//初始化这个对象
func NewPool(wn int) *Pool {
    return &Pool{workerNum: wn, road: make(chan *Car)}
}

//往channel添加具体任务
func (p *Pool) AddCar(f *Car) {
    p.road <- f
}

//goroutine去工作
func (p *Pool) work(workId int) {
    for f := range p.road {
        log.Println("workId:", workId, "start")
        f.do()
        log.Println("workId:", workId, "done")
    }
    p.wg.Done()
}

//创建goroutine等着接工作
func (p *Pool) Run() {
    for i := 0; i < p.workerNum; i++ {
        go p.work(i)
        p.wg.Add(1)
    }
    p.wg.Wait()
}

func (p *Pool) colse() {
    close(p.road)
}

func main() {
    pool := NewPool(5)
    go func() {
        //模拟要做10件事
        for i := 0; i < 10; i++ {
            car := Car{
                param: i,
            }
            pool.AddCar(&car)
        }
        pool.colse()
    }()
    pool.Run()
}

/*具体做的事通过这个来传递*/
type Car struct {
    param int
}

func (c *Car) do() error {
    log.Println(time.Now(), c.param)
    return nil
}
