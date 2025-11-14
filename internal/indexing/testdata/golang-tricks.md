# Golang Tricks Cheat Sheet

#golang #Tricks 

#### Number Manipulation

##### Parsing integer
![[parse-int-golang.png]]
```go
if numberVal, err := strconv.ParseInt(num.(string), 10, 64); err == nil {  
    return numberVal, nil  
}
```
##### Parsing float
![[parse-float-go.png]]
```go
if numberVal, err := strconv.ParseFloat(num.(string), 62); err == nil {  
    return numberVal, nil  
}
```

##### Decimal to binary
```go
strconv.FormatInt(int64(num), 2)
```

##### Generate [[random]] number between 0 and some other number
```go
rand.Intn(10) 
```

##### Get max number from a slice
```go
slices.Max(slice)
```

##### Get max number in positive and negative infinity
```go
math.Inf(-1) // In negative direction
math.Inf(1)  // In positive direction
```

#### String manipulation
##### String to integer and back 
```go
string := strconv.Itoa(5)
inti, err = strconv.Atoi("5")
```

##### String startwith and endwith
```go
import strings

strings.HasPrefix("Hello","H")
strings.HasSufffix("Hello","0")
```

##### String trimming
```go
strings.TrimSpace()
```

##### String contains
```go
strings.Contains()
```

##### String [[indexing]]
```go
strings.Index("string", "s")
strings.IndexRune("string", 62)
```

##### String [[Interpolation|interpolation]]
```go
name := "Alice"
age := 30
formatted := fmt.Sprintf("Hello, %s. You are %d years old.", name, age)

// Using strings.Builder
var builder strings.Builder
name := "Charlie"
age := 28

builder.WriteString("Hello, ")
builder.WriteString(name)
builder.WriteString(". You are ")
builder.WriteString(fmt.Sprintf("%d", age))
builder.WriteString(" years old.")
formatted := builder.String()
fmt.Println(formatted)
```


#### Date manipulation
##### Parsing `[[Date Formats#^e4a164|RFC3339]]` date
```go
parsedTime, err := time.Parse(time.RFC3339, num.(string))  
if err == nil {  
    return parsedTime.Unix(),
    parsedTime.UnixMicro(),
    parsedTime.UnixMilli(),
    parsedTime.UnixNano(),nil  
}
```

#### Collection manipulation
##### Sort an integer array
```go
import (  
"fmt"  
"sort"  
)  
  
func main() {  
intArray := []int{5, 2, 9, 1, 7}  
sort.Ints(intArray)  
fmt.Println(intArray) // Output: [1 2 5 7 9]  
}
```


#### File IO essentials

##### Read a file in golang
```go
	file, err := os.Open("data.json")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Read the file contents
	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
```

##### Read a [[JSON]] file and marshal it to a specific type
```go
	data, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	
	var person Person
	if err := json.Unmarshal(data, &person); err != nil {
		log.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Print the struct
	fmt.Printf("Person: %+v\n", person)
```
##### Write an object to a [[JSON]] format to a file
```go
	data, err := json.MarshalIndent(person, "", "  ") // Indented JSON for readability
	if err != nil {
		fmt.Printf("Failed to marshal JSON: %v\n", err)
		return
	}

	// Create the JSON file
	file, err := os.Create("output.json")
	if err != nil {
		fmt.Printf("Failed to create file: %v\n", err)
		return
	}
	defer file.Close()

	// Write the JSON data to the file
	_, err = file.Write(data)
	if err != nil {
		fmt.Printf("Failed to write to file: %v\n", err)
		return
	}

```

#### HTTP API essentials
##### [[HTTP]] [[GET]] call in [[Golang]]
![[Golang FAQs#^46e652]]

##### [[HTTP]] [[POST]] call in [[Golang]]
![[Golang FAQs#^ab5315]]
##### Read a request param from [[POST]] request
```go
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	age := r.FormValue("age")
```

##### Read an object from [[POST]] request to a specific type
```go
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	var person Person
	if err := json.Unmarshal(body, &person); err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}
```

#### Process essentials
##### Spawn a process
```go
func (n *Slave) Start() {
	wd, err := os.Getwd()  
	if err != nil {  
	    fmt.Println("Error getting current working directory:", err)  
	    return nil  
	}  
  
	binaryPath := filepath.Join(wd, "node-binary", "node")
	nodePort := fmt.Sprintf("%d", n.Port)
	masterPort := fmt.Sprintf("%d", n.master.MasterPort)

	cmd := exec.Command(n.binary, masterPort, nodePort)

	cmd.Env = os.Environ()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Start()
	if err != nil {
		fmt.Println("Error starting the process:", err)
		return
	}

	go func() {
		err = cmd.Wait()

		if err != nil {
			fmt.Println("Process finished with error:", err)
			return
		}

		fmt.Println("Process finished successfully.")
	}()
}
```

##### Spawn a [[Deamon|daemon]] process
```go
func (n *Slave) Start() {
	
	cmd := exec.Command(n.binary, masterPort, nodePort)
	
	// Daemon process  
	cmd.SysProcAttr = &syscall.SysProcAttr{  
	    Setsid: true,  
	}

}
```


#### Type check for an object
```go
reflect.ValueOf(someobj).Kind() == reflect.Array
reflect.ValueOf(someobj).Kind() == reflect.Slice
reflect.ValueOf(someobj).Kind() == reflect.Map

/*
const (  
    Invalid Kind = iota  
    Bool  
    Int    Int8    Int16    Int32    Int64    Uint    Uint8    Uint16    Uint32    Uint64    Uintptr    Float32    Float64    Complex64    Complex128    Array    Chan    Func    Interface    Map    Pointer    Slice    String    Struct    UnsafePointer)
    */
```

#### Golang [[while{}]] loop with [[for()]]
```go
int i := 0
for i < 5{
	fmt.Println(i)
	i += 1
}
```

#### Create and use [[Queue]] in [[Golang]] using standard library
```go
import (
	"container/list"
	"fmt"
)

func main() {
	// Initialize a new list, which will be used as a queue
	queue := list.New()

	// Enqueue: Add elements to the back of the queue
	queue.PushBack(10)
	queue.PushBack(20)
	queue.PushBack(30)

	// Dequeue: Remove elements from the front of the queue
	for queue.Len() > 0 {
		// Get the front element
		element := queue.Front()

		// Print the value of the element
		fmt.Println(element.Value)

		// Remove the front element from the queue
		queue.Remove(element)
	}
}
```

#### Create and use [[Graph]] in [[Golang]] using standard library
Go's standard library does not include a dedicated data structure specifically for graphs. Here’s an example of how to implement a simple graph using an adjacency list representation:
```go
import "fmt"

// Graph represents a graph using an adjacency list
type Graph struct {
	vertices map[string][]string
}

// NewGraph initializes a new graph
func NewGraph() *Graph {
	return &Graph{
		vertices: make(map[string][]string),
	}
}

// AddVertex adds a vertex to the graph
func (g *Graph) AddVertex(vertex string) {
	if _, exists := g.vertices[vertex]; !exists {
		g.vertices[vertex] = []string{}
	}
}

// AddEdge adds an edge between two vertices
func (g *Graph) AddEdge(v1, v2 string) {
	g.vertices[v1] = append(g.vertices[v1], v2)
	g.vertices[v2] = append(g.vertices[v2], v1) // Assuming an undirected graph
}

// PrintGraph prints the adjacency list of the graph
func (g *Graph) PrintGraph() {
	for vertex, edges := range g.vertices {
		fmt.Printf("%s -> %v\n", vertex, edges)
	}
}

func main() {
	// Create a new graph
	graph := NewGraph()

	// Add vertices
	graph.AddVertex("A")
	graph.AddVertex("B")
	graph.AddVertex("C")
	graph.AddVertex("D")

	// Add edges
	graph.AddEdge("A", "B")
	graph.AddEdge("A", "C")
	graph.AddEdge("B", "D")
	graph.AddEdge("C", "D")

	// Print the graph
	graph.PrintGraph()
}
```

#### Golang *fan-in* pattern

```go
func fanin(){
	c1:= generate("Hello")
	c2:= generate("There")

	fanin := make(chan string)
	go func(){
		for{
			select {
			case str1 := <-c1: fanin <- str1
			case str2 := <-c2: fanin <- str2
			}

		}
	}()

	go func(){
		for {
		fmt.Println(<-fanin)
		}
	}()

}
```

 **Analysis**

1. In **line 2, 3** we are making 2 data generators **_c1_** and **_c2_**.
2. ==In line 5 we are making the== ==**_fanin_**== ==channel which will be the converging channel that will take data from== ==**_c1_**== ==and== ==**_c2_**====.==
3. In line 9, 10 based on the data availability from channel **_c1_** and **_c2_**, the appropriate case will be selected and that same data will be pushed in to channel **_fanin_**.

#### Golang fan-out pattern

## Analysis

1. In line 21 and 26 we are declaring a **Processor** and a **Worker** struct.

> The Processor has a list of workers, which will be used as background processes to process data coming form the generator function( The data source)

2**. Line 40** defines a function to create an instance of the **Processor** and start its processing in **line 50**.

3. We interact with the processor instance with the **_postJo_**==**_b_**== ==metho==d in **line 73** which is happening in **line 85**. We are sending **11** messages to the processor instance to be processed.

4. In **line 74**, we get the message from the generator and channel it to the **jobChannel** channel in the processor instance.

5. Inside the **startProcess** method we have 2 **_selects_**. In **line 62**, we get the messages send by the generator in **line 74** inside the **_postJob_** method, only when there is a worker(**line 59**).

6. We select the worker in **line 61**(which is always the top worker of the worker slice inside the processor instance).

> In real scenario we should build a priority queue based worker pool so that the work is evenly distributed and the Processor is not blocked.
> 
> Also this setup is not backpressure aware. Line 62 can block if there is no jobs. In those cases make sure to add backpressure handling.

7. In **line 62** we give the data to the selected worker in **line 61** and also send the **done** channel of the processor instance.

8. The worker does the processing in a separate goroutine in **line 32** and _notifies the processor instance via the_ **_done_** _channel_.

9. The signal from the worker is caught in **line 64** and the worker is appended to the worker list again.

If we run the code we will see,

![](https://miro.medium.com/v2/resize:fit:1400/1*1iMHNyLjPOIFtYhyIxkitA.png)

This concludes our Fan In and Fan Out pattern. I will post another design pattern in coming posts.
