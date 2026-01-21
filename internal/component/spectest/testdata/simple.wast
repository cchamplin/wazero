;; internal/component/spectest/testdata/simple.wast
(component
  (core module (func (export "test")))
)
(assert_invalid
  (component quote "(component (export \"x\" (func 0)))")
  "unknown func"
)
