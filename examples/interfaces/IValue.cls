Option Explicit

Public Function Read(ByVal amount As Long) As Long
    Err.Raise 5, "IValue", "The interface body should not run for a Worker"
End Function
