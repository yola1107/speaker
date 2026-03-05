// 后端 API 设计 - 在线书城
// 使用 Kratos 框架 (qwen3-max-2026-01-23 生成)

// ============================================
// api/bookstore/v1/book.proto
// ============================================
syntax = "proto3";

package api.bookstore.v1;

option go_package = "bookstore/api/bookstore/v1;v1";

service BookService {
  // 获取书籍列表
  rpc ListBooks (ListBooksRequest) returns (ListBooksReply);
  // 获取书籍详情
  rpc GetBook (GetBookRequest) returns (GetBookReply);
  // 搜索书籍
  rpc SearchBooks (SearchBooksRequest) returns (SearchBooksReply);
}

message ListBooksRequest {
  int32 page = 1;
  int32 page_size = 2;
  string category = 3;
}

message ListBooksReply {
  repeated Book books = 1;
  int32 total = 2;
}

message GetBookRequest {
  int64 id = 1;
}

message GetBookReply {
  Book book = 1;
}

message SearchBooksRequest {
  string keyword = 1;
  int32 page = 2;
  int32 page_size = 3;
}

message SearchBooksReply {
  repeated Book books = 1;
  int32 total = 2;
}

message Book {
  int64 id = 1;
  string title = 2;
  string author = 3;
  double price = 4;
  string cover = 5;
  string description = 6;
  string category = 7;
}

// ============================================
// internal/service/book.go
// ============================================
package service

import (
    "context"
    v1 "bookstore/api/bookstore/v1"
    "bookstore/internal/biz"
)

type BookService struct {
    v1.UnimplementedBookServiceServer
    uc *biz.BookUsecase
}

func NewBookService(uc *biz.BookUsecase) *BookService {
    return &BookService{uc: uc}
}

func (s *BookService) ListBooks(ctx context.Context, req *v1.ListBooksRequest) (*v1.ListBooksReply, error) {
    books, total, err := s.uc.ListBooks(ctx, req.Page, req.PageSize, req.Category)
    if err != nil {
        return nil, err
    }
    return &v1.ListBooksReply{
        Books: books,
        Total: total,
    }, nil
}

func (s *BookService) GetBook(ctx context.Context, req *v1.GetBookRequest) (*v1.GetBookReply, error) {
    book, err := s.uc.GetBook(ctx, req.Id)
    if err != nil {
        return nil, err
    }
    return &v1.GetBookReply{Book: book}, nil
}

func (s *BookService) SearchBooks(ctx context.Context, req *v1.SearchBooksRequest) (*v1.SearchBooksReply, error) {
    books, total, err := s.uc.SearchBooks(ctx, req.Keyword, req.Page, req.PageSize)
    if err != nil {
        return nil, err
    }
    return &v1.SearchBooksReply{
        Books: books,
        Total: total,
    }, nil
}

// ============================================
// internal/biz/book.go
// ============================================
package biz

import (
    "context"
    v1 "bookstore/api/bookstore/v1"
)

type BookRepo interface {
    List(ctx context.Context, page, pageSize int32, category string) ([]*v1.Book, int32, error)
    GetByID(ctx context.Context, id int64) (*v1.Book, error)
    Search(ctx context.Context, keyword string, page, pageSize int32) ([]*v1.Book, int32, error)
}

type BookUsecase struct {
    repo BookRepo
}

func NewBookUsecase(repo BookRepo) *BookUsecase {
    return &BookUsecase{repo: repo}
}

func (uc *BookUsecase) ListBooks(ctx context.Context, page, pageSize int32, category string) ([]*v1.Book, int32, error) {
    return uc.repo.List(ctx, page, pageSize, category)
}

func (uc *BookUsecase) GetBook(ctx context.Context, id int64) (*v1.Book, error) {
    return uc.repo.GetByID(ctx, id)
}

func (uc *BookUsecase) SearchBooks(ctx context.Context, keyword string, page, pageSize int32) ([]*v1.Book, int32, error) {
    return uc.repo.Search(ctx, keyword, page, pageSize)
}