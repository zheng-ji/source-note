# tunny

可以固定Goroutine 数量的协程库, 不保证执行顺序
核心文件是 tunny.go 与 worker.go。

tunny 通过 reqChan 管道联系 pool 与 worker;
worker 数量与协程池的大小相等，在初始化协程池时决定；
各个 worker 竞争地获取 reqChan 中的数据，而后处理，最后返回给 pool;

```
type Pool struct {
      queuedJobs int64 // 表明 pool 当前积压的job数量
  
      ctor    func() Worker // worker 具体的构造函数
      workers []*workerWrapper // pool 实际拥有的worker
      reqChan chan workRequest // pool 与全部 worker 进行通讯的管道，worker与pool都使用相同的reqChan指针
  
      workerMut sync.Mutex // pool 进行 SetSize 操做时使用的，防止不一样协程同时对size进行操做
}

// Pool 构造函数
func New(n int, ctor func() Worker) *Pool {
      p := &Pool{
          ctor:    ctor,
          reqChan: make(chan workRequest),// 没有Buffer 的 Channel
      }
      p.SetSize(n)
      return p
}
```

Worker 接口
```
type Worker interface {
      Process(interface{}) interface{}
      BlockUntilReady()
      Interrupt()
      Terminate()
}
```

有两种Worker 
1. 闭包 Worker 
```
type closureWorker struct {
     processor func(interface{}) interface{}
}

func (w *closureWorker) Process(payload interface{}) interface{} {
	return w.processor(payload)
}
```

闭包worker，经常使用的一种worker，主要执行初始化NewFunc 时赋予它的 processeor 函数来完成；

```
func NewFunc(n int, f func(interface{}) interface{}) *Pool {
     return New(n, func() Worker {
         return &closureWorker{
             processor: f,
         }   
     })  
}
```

2. CallBack Worker

```
func NewCallback(n int) *Pool {
	return New(n, func() Worker {
		return &callbackWorker{}
	})
}

func (w *callbackWorker) Process(payload interface{}) interface{} {
	f, ok := payload.(func())
	if !ok {
		return ErrJobNotFunc
	}
	f()
	return nil
}
```

SetSize 函数会初始化 workers, 实际是 newWorkerWrapper, reqChan 用的是同一个

```
for i := lWorkers; i < n; i++ {
	p.workers = append(p.workers, newWorkerWrapper(p.reqChan, p.ctor()))
}
```

```
func newWorkerWrapper( reqChan chan<- workRequest, worker Worker,) *workerWrapper {
	w := workerWrapper{
		worker:        worker,
		interruptChan: make(chan struct{}),
		reqChan:       reqChan,
		closeChan:     make(chan struct{}),
		closedChan:    make(chan struct{}),
	}

	go w.run() // 核心是这个函数。

	return &w
}
```

以下两段代码也连着一起读才能理解·
```
func (p *Pool) Process(payload interface{}) interface{} {
	atomic.AddInt64(&p.queuedJobs, 1)

    // 步骤2, 阻塞等待reqChan 
	request, open := <-p.reqChan
    // 表示Close 
	if !open {
		panic(ErrPoolNotRunning)
	}

    // 步骤3, payload 塞入request.jobChan 触发run 下一步执行
	request.jobChan <- payload

    // 有结果返回步骤6
	payload, open = <-request.retChan
    // 表示Close , 抛出异常
	if !open {
		panic(ErrWorkerClosed)
	}

	atomic.AddInt64(&p.queuedJobs, -1)
	return payload
}

func (w *workerWrapper) run() {
	jobChan, retChan := make(chan interface{}), make(chan interface{})
	defer func() {
		w.worker.Terminate()
		close(retChan)
		close(w.closedChan)
	}()

	for {
		// NOTE: Blocking here will prevent the worker from closing down.
		w.worker.BlockUntilReady()
		select {
       // 步骤1 , Worker Run 就无脑塞入workRequest
		case w.reqChan <- workRequest{
			jobChan:       jobChan,
			retChan:       retChan,
			interruptFunc: w.interrupt,
		}:
			select {
            // 步骤4, 执行对应的Worker.Process
			case payload := <-jobChan:
				result := w.worker.Process(payload)
				select {
                // 有结果返回步骤5
				case retChan <- result:
                // 如果有中断, 重新赋值给w.interruptChan
				case <-w.interruptChan:
					w.interruptChan = make(chan struct{})
				}
			case _, _ = <-w.interruptChan:
				w.interruptChan = make(chan struct{})
			}
        // w.closeChan 作为推出信号，当w.stop w.join 会触发
		case <-w.closeChan:
			return
		}
	}
}
```

总结之后

* 步骤1, Worker Run 无脑塞入 workerRequest
* 步骤2, Pool.Process 阻塞等待 reqChan , 此时相当于挑选了一个 worker, 谁竞争拿到就用谁的 workerRequest
* 步骤3, Pool.Process 里面 将 payload 塞入workerReuqst.jobChan 触发 Run 下一步执行
* 步骤4, 执行对应的Worker.Process, worker执行下一步的Process
* 步骤5. worker.Process(payload) 有结果塞进 workerRequest.retChan
* 步骤6. pool.Process 阻塞的retChan 有响应， 就继续执行
