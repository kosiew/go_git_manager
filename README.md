# go_git_manager

go tips

go mod init dcs.com/mainproj

go run mainproj.go (a file) to run it
go build mainproj.go - to generate a binary executable


create a directory
  messages/somemessage.go
     within it - package messages

  the above can be imported
    import
      dcs.com/mainproj/messages



Capital first letter -> exported


require ...
replace ...
  to use multiple packages within one folder

constants have a "kind"i.e. integer, floating, etc
// const has enormous range and precision, calculations are in
compiler
// variables have a type

const
  iota -> increments automatically at every line


pointer
  &value
  *address

fmt.Printf（ format: "strprt is %v type %T address： %p\n", strptr,strptr,strptr）

slice vs array

1a ：= ［4]int｛｝// arrays have fixed size
//1a ：= ［]int{l,2,3,4｝ // S11ce--MoT the same as an array can change “s1ze"of S11ce

we use slice more than array

