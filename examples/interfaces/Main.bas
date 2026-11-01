Option Explicit

Public Function Main() As Long
    Dim worker As Worker, value As IValue
    Set worker = New Worker
    Set value = worker
    If Not value Is worker Then Err.Raise 5
    If worker.Read(2) <> 202 Then Err.Raise 5
    Main = value.Read(2)
End Function
