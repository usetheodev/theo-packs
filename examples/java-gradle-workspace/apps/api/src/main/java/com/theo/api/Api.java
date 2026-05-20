package com.theo.api;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

@SpringBootApplication
@RestController
public class Api {
    public static void main(String[] args) {
        SpringApplication.run(Api.class, args);
    }

    @GetMapping("/")
    public String index() {
        return "api";
    }
}
