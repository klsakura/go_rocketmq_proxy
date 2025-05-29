{
  "targets": [
    {
      "target_name": "rocketmq_addon",
      "sources": [
        "rocketmq_addon.cpp"
      ],
      "include_dirs": [
        "<!@(node -p \"require('node-addon-api').include\")",
        "."
      ],
      "defines": [
        "NAPI_DISABLE_CPP_EXCEPTIONS"
      ],
      "cflags_cc": [
        "-std=c++14",
        "-fPIC",
        "-O3"
      ],
      "conditions": [
        ["OS=='mac'", {
          "libraries": [
            "-ldl"
          ],
          "xcode_settings": {
            "GCC_ENABLE_CPP_EXCEPTIONS": "YES",
            "CLANG_CXX_LIBRARY": "libc++",
            "MACOSX_DEPLOYMENT_TARGET": "10.7",
            "OTHER_CPLUSPLUSFLAGS": [
              "-std=c++14",
              "-stdlib=libc++"
            ]
          }
        }],
        ["OS=='linux'", {
          "libraries": [
            "-ldl"
          ],
          "cflags_cc": [
            "-std=c++14",
            "-fPIC"
          ]
        }],
        ["OS=='win'", {
          "libraries": [],
          "msvs_settings": {
            "VCCLCompilerTool": {
              "ExceptionHandling": 1
            }
          }
        }]
      ]
    }
  ]
} 