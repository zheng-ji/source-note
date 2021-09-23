关于LRU 展开的各种变种的笔记
### LRU
是一种适应性Cache算法, 它结合了LRU与LFU。LRU(Least recently used)
* 其核心思想是:假设刚visit的item,很有可能在未来被revisit
* 丢弃最近最少访问的items
* 通常用双链表实现 tips: redis并没有这样做，因为这样每个key至少会多出两个指针。 redis采用的是一种近似LRU，基本思想是随机取出一些key,形成一个小的POOL，然后在pool中采用LRU策略(相关源码:redis/src/evict.c)
* 缺点:忽略了frequency, 不适合大规模扫描等情况
* LRU有一系列变种，比如LRU2, 2Q, LIRS等。

### LRU-K算法
1、算法思想LRU-K中的K代表最近使用的次数，因此LRU可以认为是LRU-1。LRU-K的主要目的是为了解决LRU算法“缓存污染”的问题，其核心思想是将“最近使用过1次”的判断标准扩展为“最近使用过K次”。
2、工作原理相比LRU，LRU-K需要多维护一个队列，用于记录所有缓存数据被访问的历史。只有当数据的访问次数达到K次的时候，才将数据放入缓存。当需要淘汰数据时，LRU-K会淘汰第K次访问时间距当前时间最大的数据。

详细实现如下
(1). 数据第一次被访问，加入到访问历史列表；
(2). 如果数据在访问历史列表里后没有达到K次访问，则按照一定规则（FIFO，LRU）淘汰；
(3). 当访问历史队列中的数据访问次数达到K次后，将数据索引从历史队列删除，将数据移到缓存队列中，并缓存此数据，缓存队列重新按照时间排序；
(4). 缓存数据队列中被再次访问后，重新排序；
(5). 需要淘汰数据时，淘汰缓存队列中排在末尾的数据，即：淘汰“倒数第K次访问离现在最久”的数据。
LRU-K具有LRU的优点，同时能够避免LRU的缺点，实际应用中LRU-2是综合各种因素后最优的选择，LRU-3或者更大的K值命中率会高，但适应性差，需要大量的数据访问才能将历史访问记录清除掉。

### Two queues（2Q）
1、算法思想该算法类似于LRU-2，
不同点在于2Q将LRU-2算法中的访问历史队列（注意这不是缓存数据的）改为一个FIFO缓存队列，即：2Q算法有两个缓存队列，一个是FIFO队列，一个是LRU队列。
2、工作原理当数据第一次访问时，2Q算法将数据缓存在FIFO队列里面，当数据第二次被访问时，则将数据从FIFO队列移到LRU队列里面，两个队列各自按照自己的方法淘汰数据。详细实现如下
(1). 新访问的数据插入到FIFO队列；
(2). 如果数据在FIFO队列中一直没有被再次访问，则最终按照FIFO规则淘汰；
(3). 如果数据在FIFO队列中被再次访问，则将数据移到LRU队列头部
(4). 如果数据在LRU队列再次被访问，则将数据移到LRU队列头部；
(5). LRU队列淘汰末尾的数据。



### ARC（Adaptive Replacement Cache）

* 整个Cache分成两部分，起始LRU和LFU各占一半，后续会动态适应调整partion的位置（记为p）
* 除此，LRU和LFU各自有一个ghost list(因此，一共4个list)
* 每次，被淘汰的item放到对应的ghost list中（ghost list只存key）, 例如：如果被evicted的item来自LRU的部分， 则该item对应的key会被放入LRU对应的ghost list
* 第一次cache miss, 则会放入LRU
* 如果cache hit, 如果LFU中没有，则放入LFU
* 如果cache miss, 但在ghost list中命中，这说明对应的cache如果再大一丁点儿就好了： 如果存在于LRU ghost list, 则p=p+1；否则存在于LFU ghost list, p=p-1.
* 也就是说，利用这种适应机制，当系统趋向于访问最近的内容，会更多地命中LRU ghost list，这样会增大LRU的空间； 当系统趋向于访问最频繁的内容，会更多地命中LFU ghost list，这样会增加LFU的空间.


### LFU(Least-frequently used)
* 其核心思想是:假设visit次数越多的item,很有可能在未来被revisit
* 适应大规模扫描
* 对热点友好
* 缺点:忽略了recency, 可能会积累不再使用的数据 tips: redis4.0开始支持了LFU,例如volatile-lfu, allkeys-lfu配置选项


