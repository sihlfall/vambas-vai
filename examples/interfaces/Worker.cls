Option Explicit
Implements IValue

Private Function IValue_Read(ByVal amount As Long) As Long
    IValue_Read = amount + 40
End Function

Public Function Read(ByVal amount As Long) As Long
    Read = amount + 200
End Function
