Option Explicit

Public Function Main() As Long
    Dim counter As New Counter
    Dim late As Object
    Dim boxed As Variant
    Set late = counter
    Set boxed = late
    late.Increment
    boxed.Increment
    With boxed
        Set boxed = Nothing
        Main = .Value
    End With
    Main = counter
End Function
