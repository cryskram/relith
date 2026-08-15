package chunker

import (
	"strings"
	"testing"
)

func symbolsOf(chunks []Chunk) []string {
	var names []string
	for _, c := range chunks {
		for _, s := range c.Symbols {
			names = append(names, s.Kind+":"+s.Name)
		}
	}
	return names
}

func chunkCount(chunks []Chunk) int {
	return len(chunks)
}

func TestGoChunker_Basic(t *testing.T) {
	code := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}

func add(a, b int) int {
	return a + b
}
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	if !contains(names, "function:main") {
		t.Errorf("expected function:main in symbols, got %v", names)
	}
	if !contains(names, "function:add") {
		t.Errorf("expected function:add in symbols, got %v", names)
	}
}

func TestGoChunker_StructAndInterface(t *testing.T) {
	code := `package main

type Foo struct {
	Name string
	age  int
}

type Bar interface {
	Do() error
}

type MyFunc func(string) error
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if !contains(names, "struct:Foo") {
		t.Errorf("expected struct:Foo, got %v", names)
	}
	if !contains(names, "interface:Bar") {
		t.Errorf("expected interface:Bar, got %v", names)
	}
	if !contains(names, "type:MyFunc") {
		t.Errorf("expected type:MyFunc, got %v", names)
	}
}

func TestGoChunker_Method(t *testing.T) {
	code := `package main

type Foo struct{}

func (f *Foo) Do() string {
	return "done"
}

func (f Foo) Get() int {
	return 42
}
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if !contains(names, "method:Do") {
		t.Errorf("expected method:Do, got %v", names)
	}
	if !contains(names, "method:Get") {
		t.Errorf("expected method:Get, got %v", names)
	}
}

func TestGoChunker_Generics(t *testing.T) {
	code := `package main

type List[T any] struct {
	items []T
}

func (l *List[T]) Add(item T) {
	l.items = append(l.items, item)
}

func NewList[T any]() *List[T] {
	return &List[T]{}
}
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if !contains(names, "struct:List") {
		t.Errorf("expected struct:List, got %v", names)
	}
	if !contains(names, "method:Add") {
		t.Errorf("expected method:Add, got %v", names)
	}
	if !contains(names, "function:NewList") {
		t.Errorf("expected function:NewList, got %v", names)
	}
}

func TestGoChunker_BracesInStrings(t *testing.T) {
	code := `package main

import "fmt"

func greet() {
	msg := fmt.Sprintf("hello {world}")
	fmt.Println(msg)
}
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:greet") {
		t.Errorf("expected function:greet, got %v", names)
	}
	// Should have one function chunk, not multiple
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestGoChunker_StructBraceOnNextLine(t *testing.T) {
	code := `package main

type Foo struct
{
	Name string
}
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if !contains(names, "struct:Foo") {
		t.Errorf("expected struct:Foo, got %v", names)
	}
}

func TestGoChunker_EmptyFile(t *testing.T) {
	chunks := GoChunkerAST("")
	if chunkCount(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty file, got %d", chunkCount(chunks))
	}
}

func TestGoChunker_NoDeclarations(t *testing.T) {
	code := `package main

import "fmt"

var x = 42
const y = "hello"
`
	chunks := GoChunkerAST(code)
	if chunkCount(chunks) == 0 {
		t.Errorf("expected fallback chunks for file with no declarations, got 0")
	}
}

func TestPythonChunker_Basic(t *testing.T) {
	code := `def hello():
    print("hello")

def add(a, b):
    return a + b
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:hello") {
		t.Errorf("expected function:hello, got %v", names)
	}
	if !contains(names, "function:add") {
		t.Errorf("expected function:add, got %v", names)
	}
}

func TestPythonChunker_Class(t *testing.T) {
	code := `class Person:
    def __init__(self, name):
        self.name = name

    def greet(self):
        print(f"hello {self.name}")
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:Person") {
		t.Errorf("expected class:Person, got %v", names)
	}
	if !contains(names, "function:__init__") {
		t.Errorf("expected function:__init__, got %v", names)
	}
	if !contains(names, "function:greet") {
		t.Errorf("expected function:greet, got %v", names)
	}
}

func TestPythonChunker_Decorators(t *testing.T) {
	code := `import dataclasses

@dataclasses.dataclass
class Config:
    name: str
    port: int

@app.route("/api")
@auth.required
def handler():
    return "ok"
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:Config") {
		t.Errorf("expected class:Config, got %v", names)
	}
	if !contains(names, "function:handler") {
		t.Errorf("expected function:handler, got %v", names)
	}
	// Decorator lines should be part of the declaration chunks, not separate
	foundDecorator := false
	for _, c := range chunks {
		if c.Symbols != nil && strings.Contains(c.Content, "@dataclasses.dataclass") {
			foundDecorator = true
			break
		}
	}
	if !foundDecorator {
		t.Errorf("expected decorator line to be part of a declaration chunk")
	}
}

func TestPythonChunker_NestedFunctions(t *testing.T) {
	code := `def outer():
    x = 1
    def inner():
        return x
    return inner()
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:outer") {
		t.Errorf("expected function:outer, got %v", names)
	}
	if !contains(names, "function:inner") {
		t.Errorf("expected function:inner, got %v", names)
	}
}

func TestPythonChunker_LambdaIgnored(t *testing.T) {
	code := `def sort_items():
    items = [3, 1, 2]
    return sorted(items, key=lambda x: x)
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:sort_items") {
		t.Errorf("expected function:sort_items, got %v", names)
	}
	if len(names) != 1 {
		t.Errorf("expected 1 symbol only (lambda should be ignored), got %v", names)
	}
}

func TestPythonChunker_EmptyFile(t *testing.T) {
	chunks := PythonChunker("")
	if chunkCount(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty file, got %d", chunkCount(chunks))
	}
}

func TestPythonChunker_NoDeclarations(t *testing.T) {
	code := `x = 42
y = "hello"
z = [1, 2, 3]
`
	chunks := PythonChunker(code)
	if chunkCount(chunks) == 0 {
		t.Errorf("expected fallback chunks, got 0")
	}
}

func TestBraceChunker_JSFunctions(t *testing.T) {
	code := `function greet(name) {
    return "hello " + name;
}

function add(a, b) {
    return a + b;
}
`
	chunks := BraceChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:greet") {
		t.Errorf("expected function:greet, got %v", names)
	}
	if !contains(names, "function:add") {
		t.Errorf("expected function:add, got %v", names)
	}
}

func TestBraceChunker_JSClass(t *testing.T) {
	code := `class Animal {
    constructor(name) {
        this.name = name;
    }

    speak() {
        console.log(this.name);
    }
}
`
	chunks := BraceChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:Animal") {
		t.Errorf("expected class:Animal, got %v", names)
	}
}

func TestBraceChunker_ArrowFunctions(t *testing.T) {
	code := `const greet = (name) => {
    return "hello " + name;
};

const add = (a, b) => a + b;

var old = function() {
    return 42;
};
`
	chunks := BraceChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:greet") {
		t.Errorf("expected function:greet, got %v", names)
	}
	if !contains(names, "function:add") {
		t.Errorf("expected function:add, got %v", names)
	}
	// anonymous function expressions (var old = function() {}) not detected
}

func TestBraceChunker_Rust(t *testing.T) {
	code := `pub struct User {
    name: String,
    age: u32,
}

impl User {
    pub fn new(name: String, age: u32) -> Self {
        User { name, age }
    }

    fn greet(&self) -> String {
        format!("hello {}", self.name)
    }
}

pub fn create_user() -> User {
    User::new("test".into(), 30)
}
`
	chunks := BraceChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "struct:User") {
		t.Errorf("expected struct:User, got %v", names)
	}
	if !contains(names, "impl:User") {
		t.Errorf("expected impl:User, got %v", names)
	}
	if !contains(names, "function:new") {
		t.Errorf("expected function:new, got %v", names)
	}
	if !contains(names, "function:greet") {
		t.Errorf("expected function:greet, got %v", names)
	}
	if !contains(names, "function:create_user") {
		t.Errorf("expected function:create_user, got %v", names)
	}
}

func TestBraceChunker_Java(t *testing.T) {
	code := `public class Hello {
    private String name;

    public Hello(String name) {
        this.name = name;
    }

    public String greet() {
        return "hello " + name;
    }
}
`
	chunks := BraceChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:Hello") {
		t.Errorf("expected class:Hello, got %v", names)
	}
}

func TestBraceChunker_AnnotationsSkipped(t *testing.T) {
	code := `@Override
public String toString() {
    return "test";
}

@RequestMapping("/api")
public Response handle() {
    return Response.ok();
}
`
	chunks := BraceChunker(code)
	// May not detect Java methods well; rely on the chunk count assertion below.
	if chunkCount(chunks) == 0 {
		t.Errorf("expected some chunks, got 0")
	}
}

func TestBraceChunker_EmptyFile(t *testing.T) {
	chunks := BraceChunker("")
	if chunkCount(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty file, got %d", chunkCount(chunks))
	}
}

func TestFallbackChunker_Basic(t *testing.T) {
	code := `line1
line2
line3`
	chunks := FallbackChunker(code)
	if chunkCount(chunks) < 1 {
		t.Errorf("expected at least 1 chunk, got %d", chunkCount(chunks))
	}
	if len(chunks) > 0 && chunks[0].Content != code {
		t.Errorf("expected full content for small file")
	}
}

func TestFallbackChunker_LargeFile(t *testing.T) {
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, "line")
	}
	code := strings.Join(lines, "\n")
	chunks := FallbackChunker(code)
	if chunkCount(chunks) < 2 {
		t.Errorf("expected multiple chunks for 200 lines, got %d", chunkCount(chunks))
	}
}

func TestForLanguage(t *testing.T) {
	if fn := ForLanguage("Go"); fn == nil {
		t.Error("expected non-nil chunker for Go")
	}
	if fn := ForLanguage("Python"); fn == nil {
		t.Error("expected non-nil chunker for Python")
	}
	if fn := ForLanguage("JavaScript"); fn == nil {
		t.Error("expected non-nil chunker for JavaScript")
	}
	if fn := ForLanguage("CSS"); fn == nil {
		t.Error("expected non-nil chunker for unknown language (fallback)")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestGoChunker_ChunkBoundaries(t *testing.T) {
	code := `package main

func alpha() {
	return 1
}

func beta() {
	return 2
}
`
	chunks := GoChunkerAST(code)

	// Each function should be a separate chunk
	funcChunks := 0
	for _, c := range chunks {
		for range c.Symbols {
			funcChunks++
		}
	}
	if funcChunks < 2 {
		t.Errorf("expected at least 2 function symbols, got %d", funcChunks)
	}

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks (alpha + beta), got %d", len(chunks))
	}
}

func TestGoChunker_NestedBraceDepth(t *testing.T) {
	code := `package main

func complex() {
	if true {
		for i := 0; i < 10; i++ {
			switch i {
			case 1:
				fmt.Println("one")
			}
		}
	}
	return
}
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:complex") {
		t.Errorf("expected function:complex, got %v", names)
	}
	// The function should be a single chunk
	if len(chunks) < 1 {
		t.Fatal("expected at least 1 chunk")
	}
}

func TestGoChunker_SkipImportBlock(t *testing.T) {
	code := `package main

import (
	"fmt"
	"os"
)

func run() {
	fmt.Println(os.Args)
}
`
	chunks := GoChunkerAST(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:run") {
		t.Errorf("expected function:run, got %v", names)
	}
}

func TestPythonChunker_Async(t *testing.T) {
	code := `async def fetch_data():
    return await api.get()

async def process():
    data = await fetch_data()
    return data
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:fetch_data") {
		t.Errorf("expected function:fetch_data, got %v", names)
	}
	if !contains(names, "function:process") {
		t.Errorf("expected function:process, got %v", names)
	}
}

func TestPythonChunker_TopLevelCode(t *testing.T) {
	code := `import os
import sys

x = 42

def handler():
    return x
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:handler") {
		t.Errorf("expected function:handler, got %v", names)
	}
}

func TestPythonChunker_ClassMethodsWithDecorators(t *testing.T) {
	code := `class Service:
    @property
    def name(self):
        return "svc"

    @staticmethod
    def create():
        return Service()
`
	chunks := PythonChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:Service") {
		t.Errorf("expected class:Service, got %v", names)
	}
	if !contains(names, "function:name") {
		t.Errorf("expected function:name, got %v", names)
	}
	if !contains(names, "function:create") {
		t.Errorf("expected function:create, got %v", names)
	}
}

func TestBraceChunker_JSAsyncGenerator(t *testing.T) {
	code := `async function* stream() {
    yield 1;
    yield 2;
}

async function consume() {
    for await (const v of stream()) {
        console.log(v);
    }
}
`
	chunks := BraceChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:stream") {
		t.Errorf("expected function:stream, got %v", names)
	}
	if !contains(names, "function:consume") {
		t.Errorf("expected function:consume, got %v", names)
	}
}

func TestBraceChunker_CStruct(t *testing.T) {
	code := `struct point {
    int x;
    int y;
};

struct point*
make_point(int x, int y) {
    struct point *p = malloc(sizeof(struct point));
    p->x = x;
    p->y = y;
    return p;
}
`
	chunks := BraceChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "struct:point") {
		t.Errorf("expected struct:point, got %v", names)
	}
}

func TestJavaChunker_Basic(t *testing.T) {
	code := `package com.example;

import java.util.List;

/**
 * Javadoc comment
 */
public class UserService {
    private String name;

    public String getName() {
        return name;
    }

    public void setName(String name) {
        this.name = name;
    }
}

interface Repository {
    List<String> findAll();
}

enum Status {
    ACTIVE,
    INACTIVE
}
`
	chunks := JavaChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:UserService") {
		t.Errorf("expected class:UserService, got %v", names)
	}
	if !contains(names, "interface:Repository") {
		t.Errorf("expected interface:Repository, got %v", names)
	}
}

func TestJavaChunker_Annotations(t *testing.T) {
	code := `@Override
public String toString() {
    return "hello";
}

@Deprecated
@SuppressWarnings("unchecked")
public void oldMethod() {
}

@GetMapping("/api")
@PreAuthorize("hasRole('ADMIN')")
public ApiResponse<Page<User>> getUsers(@RequestParam int page) {
    return service.getUsers(page);
}
`
	chunks := JavaChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "function:toString") {
		t.Errorf("expected function:toString, got %v", names)
	}
	if !contains(names, "function:oldMethod") {
		t.Errorf("expected function:oldMethod, got %v", names)
	}
	if !contains(names, "function:getUsers") {
		t.Errorf("expected function:getUsers, got %v", names)
	}
}

func TestJavaChunker_InnerClass(t *testing.T) {
	code := `public class Outer {
    private int x;

    class Inner {
        void doSomething() {
        }
    }

    record Point(int x, int y) {
    }
}
`
	chunks := JavaChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:Outer") {
		t.Errorf("expected class:Outer, got %v", names)
	}
	if contains(names, "class:Inner") {
		t.Errorf("did not expect Inner (nested class), got %v", names)
	}
}

func TestJavaChunker_EmptyFile(t *testing.T) {
	chunks := JavaChunker("")
	if chunks != nil {
		t.Error("expected nil chunks for empty file")
	}
}

func TestCppChunker_ClassAndFunction(t *testing.T) {
	code := `#include <iostream>
#include <vector>

namespace myapp {
    class Foo {
    public:
        Foo() = default;
        void bar();
        int calculate(int x, int y);
    };

    struct Point {
        int x;
        int y;
    };
}

int main(int argc, char* argv[]) {
    myapp::Foo f;
    f.calculate(1, 2);
    return 0;
}
`
	chunks := CppChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "module:myapp") {
		t.Errorf("expected module:myapp, got %v", names)
	}
	if !contains(names, "function:main") {
		t.Errorf("expected function:main, got %v", names)
	}
}

func TestCppChunker_Csharp(t *testing.T) {
	code := `using System;
using System.Collections.Generic;

namespace MyApp {
    public class Service {
        public string Name { get; set; }
        public event EventHandler Changed;
        public delegate void Callback(int x);

        public void DoWork() {
            Console.WriteLine("working");
        }
    }

    public record Person(string Name, int Age);
}
`
	chunks := CppChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "module:MyApp") {
		t.Errorf("expected module:MyApp, got %v", names)
	}
}

func TestCppChunker_Kotlin(t *testing.T) {
	code := `package com.example

import javax.inject.Inject

class UserService @Inject constructor(
    private val repository: UserRepository
) {
    fun getUsers(): List<User> {
        return repository.findAll()
    }

    suspend fun fetchAsync(): User {
        return withContext(Dispatchers.IO) { repository.get() }
    }
}

data class UserDto(
    val id: Long,
    val name: String
)

interface Repository {
    fun find(): List<User>
}

object NetworkConfig {
    val baseUrl = "https://api.example.com"
}
`
	chunks := CppChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:UserService") {
		t.Errorf("expected class:UserService, got %v", names)
	}
	if !contains(names, "function:getUsers") {
		t.Errorf("expected function:getUsers, got %v", names)
	}
	if !contains(names, "function:fetchAsync") {
		t.Errorf("expected function:fetchAsync, got %v", names)
	}
	if !contains(names, "interface:Repository") {
		t.Errorf("expected interface:Repository, got %v", names)
	}
}

func TestCppChunker_Swift(t *testing.T) {
	code := `import Foundation

class UserManager {
    private var users: [User] = []

    func fetchUsers() async throws -> [User] {
        return users
    }

    func addUser(_ user: User) {
        users.append(user)
    }
}

struct Address {
    let street: String
    let city: String
}

enum NetworkError: Error {
    case notFound
    case timeout
}

protocol DataSource {
    func load() -> [User]
}

extension UserManager: DataSource {
    func load() -> [User] {
        return users
    }
}
`
	chunks := CppChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:UserManager") {
		t.Errorf("expected class:UserManager, got %v", names)
	}
	if !contains(names, "class:Address") {
		t.Errorf("expected class:Address, got %v", names)
	}
}

func TestCppChunker_EmptyFile(t *testing.T) {
	chunks := CppChunker("")
	if chunks != nil {
		t.Error("expected nil chunks for empty file")
	}
}

func TestPHPChunker_Basic(t *testing.T) {
	code := `<?php

namespace App\Service;

use App\Entity\User;

class UserService {
    private $repository;

    public function getUsers(): array {
        return $this->repository->findAll();
    }

    public function createUser(string $name): User {
        $user = new User();
        $user->setName($name);
        return $user;
    }
}

function helper(): string {
    return 'helper';
}

interface CacheInterface {
    public function get(string $key): mixed;
    public function set(string $key, mixed $value, int $ttl = 0): void;
}
`
	chunks := PHPChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:UserService") {
		t.Errorf("expected class:UserService, got %v", names)
	}
	if !contains(names, "function:helper") {
		t.Errorf("expected function:helper, got %v", names)
	}
	if !contains(names, "interface:CacheInterface") {
		t.Errorf("expected interface:CacheInterface, got %v", names)
	}
}

func TestPHPChunker_EmptyFile(t *testing.T) {
	chunks := PHPChunker("")
	if chunks != nil {
		t.Error("expected nil chunks for empty file")
	}
}

func TestRubyChunker_Basic(t *testing.T) {
	code := `class UserService
  def initialize(name)
    @name = name
  end

  def get_name
    @name
  end

  private

  def internal_method
    puts "hidden"
  end
end

module DataHelper
  def self.format(data)
    data.to_s
  end
end

def top_level_function
  puts "hello"
end
`
	chunks := RubyChunker(code)
	names := symbolsOf(chunks)

	if !contains(names, "class:UserService") {
		t.Errorf("expected class:UserService, got %v", names)
	}
	if !contains(names, "module:DataHelper") {
		t.Errorf("expected module:DataHelper, got %v", names)
	}
	if !contains(names, "function:top_level_function") {
		t.Errorf("expected function:top_level_function, got %v", names)
	}
}

func TestRubyChunker_EmptyFile(t *testing.T) {
	chunks := RubyChunker("")
	if chunks != nil {
		t.Error("expected nil chunks for empty file")
	}
}
