package result

import "container/heap"

type ResultData struct {
	Id       int
	Position string
	Count    int
	Avg      float64
	Ratio    float64
	Index    int
}

// A ResultList implements heap.Interface and holds Items.
type ResultList []*ResultData

func (rl ResultList) Len() int { return len(rl) }

func (rl ResultList) Less(i, j int) bool {
	// We want Pop to give us the highest, not lowest, priority so we use greater than here.
	return rl[i].Count > rl[j].Count
}

func (rl ResultList) Swap(i, j int) {
	rl[i], rl[j] = rl[j], rl[i]
	rl[i].Index = i
	rl[j].Index = j
}

func (rl *ResultList) Push(x any) {
	n := len(*rl)
	data := x.(*ResultData)
	data.Index = n
	*rl = append(*rl, data)
}

func (rl *ResultList) Pop() any {
	old := *rl
	n := len(old)
	data := old[n-1]
	old[n-1] = nil  // avoid memory leak
	data.Index = -1 // for safety
	*rl = old[0 : n-1]
	return data
}

// update modifies the priority and value of an Item in the queue.
func (rl *ResultList) update(data *ResultData, id int, position string, count int, avg float64, ratio float64) {
	data.Id = id
	data.Position = position
	data.Count = count
	data.Avg = avg
	data.Ratio = ratio
	heap.Fix(rl, data.Index)
}
