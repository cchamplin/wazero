(component
  (core module $m
    (func $add (export "add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.add
    )
    (func $trap (export "trap") (unreachable))
  )
  (core instance $i (instantiate $m))
  (type $ft (func (param "a" s32) (param "b" s32) (result s32)))
  (func $add (type $ft) (canon lift (core func $i "add")))
  (export "add" (func $add))
  (type $ft2 (func))
  (func $trap (type $ft2) (canon lift (core func $i "trap")))
  (export "trap" (func $trap))
)
(invoke "add" (s32.const 1) (s32.const 2))
(assert_return (invoke "add" (s32.const 1) (s32.const 2)) (s32.const 3))
(assert_return (invoke "add" (s32.const 0) (s32.const 0)) (s32.const 0))
(assert_return (invoke "add" (s32.const -1) (s32.const 1)) (s32.const 0))
(assert_trap (invoke "trap") "unreachable")
