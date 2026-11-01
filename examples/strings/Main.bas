Option Explicit

Private Const Prefix As String = "Hello" & ", "

Public Function Main() As Long
    Dim name As String
    name = "VBA"
    AppendMark name
    Main = Len(Greeting(name))
End Function

Private Function Greeting(ByVal name As String) As String
    Greeting = Prefix & name
End Function

Private Sub AppendMark(ByRef value As String)
    value = value & "!"
End Sub
