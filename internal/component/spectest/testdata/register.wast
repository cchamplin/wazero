(component $A
  (core module $m
    (func $add (export "add") (param i32 i32) (result i32)
      local.get 0
      local.get 1
      i32.add
    )
  )
  (core instance $i (instantiate $m))
  (type $ft (func (param "a" s32) (param "b" s32) (result s32)))
  (func $add (type $ft) (canon lift (core func $i "add")))
  (export "add" (func $add))
)
(register "A")
(assert_return (invoke $A "add" (s32.const 10) (s32.const 20)) (s32.const 30))
