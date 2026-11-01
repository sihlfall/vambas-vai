Option Explicit

Public Function Main() As Long
    Dim counter As Counter
    Set counter = New Counter
    counter.Increment
    Main = counter.Value
End Function
