#!/bin/bash

# 测试语言包功能的脚本

set -e

echo "Building anycode-server..."
cd "$(dirname "$0")/.."
make build

echo "Testing builtin language packages..."

# 创建临时测试文件
TEST_DIR=$(mktemp -d)
echo "Using test directory: $TEST_DIR"

# 创建Java测试文件
cat > "$TEST_DIR/Test.java" << 'EOF'
public class Test {
    private String name;
    
    public Test(String name) {
        this.name = name;
    }
    
    public void sayHello() {
        System.out.println("Hello, " + name);
    }
    
    public static void main(String[] args) {
        Test test = new Test("World");
        test.sayHello();
    }
}
EOF

# 创建Go测试文件
cat > "$TEST_DIR/test.go" << 'EOF'
package main

import "fmt"

type Person struct {
    Name string
    Age  int
}

func (p Person) SayHello() {
    fmt.Printf("Hello, I'm %s\n", p.Name)
}

func main() {
    person := Person{
        Name: "Alice",
        Age:  30,
    }
    person.SayHello()
}
EOF

# 创建Python测试文件
cat > "$TEST_DIR/test.py" << 'EOF'
class Person:
    def __init__(self, name, age):
        self.name = name
        self.age = age
    
    def say_hello(self):
        print(f"Hello, I'm {self.name}")
    
    def get_age(self):
        return self.age

def main():
    person = Person("Bob", 25)
    person.say_hello()
    print(f"Age: {person.get_age()}")

if __name__ == "__main__":
    main()
EOF

echo "Created test files:"
echo "  - $TEST_DIR/Test.java"
echo "  - $TEST_DIR/test.go"  
echo "  - $TEST_DIR/test.py"

echo ""
echo "Starting language server..."
echo "You can now test the following features:"
echo "  1. Open the test files in your editor"
echo "  2. Test go-to-definition on symbols"
echo "  3. Test code completion"
echo "  4. Test find references"
echo "  5. Test outline/symbols"

echo ""
echo "Server command line:"
echo "./anycode-server -mode=stdio"

echo ""
echo "Or with external packages:"
echo "./anycode-server -mode=stdio -packages=../"

echo ""
echo "Cleanup test files when done:"
echo "rm -rf $TEST_DIR"

# 可选：启动服务器（如果需要交互测试）
# ./anycode-server -mode=stdio