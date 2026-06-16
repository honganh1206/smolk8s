package worker

type (
	Queue struct {
		// Front/left
		start *node
		// Back/right
		end    *node
		length int
	}
	// Linked list
	node struct {
		value any
		next  *node
	}
)

// Create a new queue
func NewQueue() *Queue {
	return &Queue{nil, nil, 0}
}

// Take this item off the start (left side) of the queue
func (q *Queue) Dequeue() any {
	if q.length == 0 {
		return nil
	}

	n := q.start
	if q.length == 1 {
		// Last item
		q.start = nil
		q.end = nil
	} else {
		// Second one (from left to right) as start node
		q.start = q.start.next
	}
	q.length--
	return n.value
}

// Put an item on the end (right side) of the queue
func (q *Queue) Enqueue(value any) {
	n := &node{value, nil}
	if q.length == 0 {
		q.start = n
		q.end = n
	} else {
		// Link the CURRENT end node to the new node
		q.end.next = n
		// Move end to the new last node
		q.end = n
	}
	q.length++
}

// Return the number of items in the queue
func (q *Queue) Len() int {
	return q.length
}

// Return the first item in the queue without removing it
func (q *Queue) Peek() any {
	if q.length == 0 {
		return nil
	}
	return q.start.value
}
